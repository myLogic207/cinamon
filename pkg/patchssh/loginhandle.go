package patchssh

import (
	"bufio"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"

	"golang.org/x/crypto/ssh"
)

const (
	SSH_KEY_SIZE = 4096
	SSH_KEY_TYPE = "ed25519"
)

var (
	SSH_KEY_TYPES      = []string{"ssh-ed25519", "ssh-rsa"}
	ErrKeyFileIsDir    = errors.New("key file is a directory")
	ErrKeyFileNoPEM    = errors.New("key file has no PEM block")
	ErrKeyFileNotRSA   = errors.New("key file is not a RSA key")
	ErrKeyNotSupported = errors.New("key type not supported")
)

type AuthManager struct {
	signer    ssh.Signer
	knownKeys map[string]bool
}

func NewAuthManager(keyFilePath string, knownKeysPath string) (*AuthManager, error) {
	privateKey, err := ensureKeyFile(keyFilePath)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return nil, err
	}
	knownKeys := make(map[string]bool)
	if knownKeysPath != "" {
		if !path.IsAbs(knownKeysPath) {
			pwd, _ := os.Getwd()
			knownKeysPath = path.Join(pwd, knownKeysPath)
		}
		knownKeys, err = loadKnownKeys(knownKeysPath)
		if err != nil {
			return nil, err
		}
	}
	if err != nil {
		return nil, err
	}
	return &AuthManager{
		signer:    signer,
		knownKeys: knownKeys,
	}, nil
}

func ensureKeyFile(filePath string) ([]byte, error) {
	if !path.IsAbs(filePath) {
		pwd, _ := os.Getwd()
		filePath = path.Join(pwd, filePath)
	}
	// make sure all directories exist
	if err := os.MkdirAll(path.Dir(filePath), 0755); err != nil {
		return nil, err
	}
	if stat, err := os.Stat(filePath); err != nil && errors.Is(err, os.ErrNotExist) {
		return createKeyPair(filePath)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	} else if stat.IsDir() {
		return nil, ErrKeyFileIsDir
	} else if stat.Size() == 0 {
		if err := os.Remove(filePath); err != nil {
			return nil, err
		}
		return createKeyPair(filePath)
	} else {
		return os.ReadFile(filePath)
	}
}

func createKeyPair(filePath string) ([]byte, error) {
	var rawPrivateKey any
	var rawPublicKey any

	if SSH_KEY_TYPE == "rsa" {
		rawPrivateKey, err := generatePrivateKey(SSH_KEY_SIZE)
		if err != nil {
			return nil, err
		}
		rawPublicKey = &rawPrivateKey.PublicKey
	} else if SSH_KEY_TYPE == "ed25519" {
		var err error
		rawPublicKey, rawPrivateKey, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
	}
	// generate public key
	publicKey, err := ssh.NewPublicKey(rawPublicKey)
	if err != nil {
		return nil, err
	}

	// generate private key
	pemBlock, err := ssh.MarshalPrivateKey(crypto.PrivateKey(rawPrivateKey), "")
	// bytes, err := x509.MarshalPKCS8PrivateKey(rawPrivateKey)
	if err != nil {
		return nil, err
	}
	// pemBlock := &pem.Block{
	// 	Type:    "RSA PRIVATE KEY",
	// 	Bytes:   bytes,
	// 	Headers: nil,
	// }
	privateKeyBytes := pem.EncodeToMemory(pemBlock)

	// write private key to file
	if err := writeKeyToFile(filePath, privateKeyBytes); err != nil {
		return nil, err
	}
	// write public key to file
	if err := writeKeyToFile(filePath+".pub", ssh.MarshalAuthorizedKey(publicKey)); err != nil {
		return nil, err
	}
	return privateKeyBytes, nil
}

func generatePrivateKey(size int) (*rsa.PrivateKey, error) {
	rawKey, err := rsa.GenerateKey(rand.Reader, size)
	if err != nil {
		return nil, err
	}
	// validate key
	if err := rawKey.Validate(); err != nil {
		return nil, err
	}
	return rawKey, nil
}

func writeKeyToFile(filePath string, keyBytes []byte) error {
	err := os.WriteFile(filePath, keyBytes, 0600)
	if err != nil {
		return err
	}
	return nil
}

func loadKnownKeys(filePath string) (map[string]bool, error) {
	if !path.IsAbs(filePath) {
		pwd, _ := os.Getwd()
		filePath = path.Join(pwd, filePath)
	}
	// make sure all directories exist
	if err := os.MkdirAll(path.Dir(filePath), 0755); err != nil {
		return nil, err
	}
	knownKeys := make(map[string]bool)
	if stat, err := os.Stat(filePath); err != nil && errors.Is(err, os.ErrNotExist) {
		return knownKeys, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	} else if stat.IsDir() {
		return nil, ErrKeyFileIsDir
	} else if stat.Size() == 0 {
		return knownKeys, nil
	}
	// parse file line by line and add to knownKeys
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		keyBytes := scanner.Bytes()
		if len(keyBytes) == 0 {
			continue
		}
		pubKey, _, _, _, err := ssh.ParseAuthorizedKey(keyBytes)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(SSH_KEY_TYPES, pubKey.Type()) {
			println(fmt.Sprintf("Key type %s not supported", pubKey.Type()))
			return nil, ErrKeyNotSupported
		}
		knownKeys[string(pubKey.Marshal())] = true
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return knownKeys, nil
}

func (km *AuthManager) PublicKeyCallback(c ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
	if !slices.Contains(SSH_KEY_TYPES, pubKey.Type()) {
		return nil, ErrKeyNotSupported
	}
	if km.knownKeys[string(pubKey.Marshal())] {
		return &ssh.Permissions{
			Extensions: map[string]string{
				"pubkey-fp":               ssh.FingerprintSHA256(pubKey),
				"permit-X11-forwarding":   "true",
				"permit-agent-forwarding": "true",
			},
		}, nil
	}
	return nil, errors.New("unknown public key for " + c.User())
}

func (km *AuthManager) NoAuthCallback(conn ssh.ConnMetadata) (*ssh.Permissions, error) {
	// allowing guest user access
	if conn.User() == "guest" {
		return &ssh.Permissions{
			Extensions: map[string]string{
				"pubkey-fp": "guest",
			},
		}, nil
	}
	return nil, errors.New("authentication required")
}

func (km *AuthManager) PasswordAuth(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	// ignore password for guest user
	if conn.User() == "guest" {
		return &ssh.Permissions{
			Extensions: map[string]string{
				"pubkey-fp": "guest",
			},
		}, nil
	}
	return nil, errors.New("password authentication not allowed")
}

func (km *AuthManager) KeyboardInteractiveAuth(conn ssh.ConnMetadata, challenge ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
	// simple yes or no question
	answers, err := challenge(conn.User(), "", []string{"Do you want to connect as guest (yes/no)?"}, []bool{true})
	if err != nil {
		return nil, err
	}
	if len(answers) != 1 || answers[0] != "yes" {
		return nil, errors.New("connection aborted")
	}
	return &ssh.Permissions{
		Extensions: map[string]string{
			"pubkey-fp": "guest",
		},
	}, nil
}

func (km *AuthManager) Banner(conn ssh.ConnMetadata) string {
	builder := &strings.Builder{}
	length := 79
	endLine := fmt.Sprintf(".%s.\n", strings.Repeat("-", length-2))
	builder.WriteString(endLine)
	builder.WriteString(formatLine(fmt.Sprintf("Hello %s!", conn.User()), length, 0, 4, true))
	builder.WriteString(formatLine("Welcome to patchssh!", length, 0, 4, true))
	builder.WriteString(formatLine("", length, 1, 0, true))
	builder.WriteString(formatLine("!This is a test banner!", length, 1, 4, true))
	builder.WriteString(endLine)
	return builder.String()
}

// fills the string up to the given length with spaces
// orientation: 0 = left, 1 = center, 2 = right
// spacing: number of spaces between text and border, aka opposite of orientation
func formatLine(raw string, length int, orientation int, spacing int, border bool) string {
	builder := &strings.Builder{}
	if border {
		length -= 2
		builder.WriteString("|")
	}
	if len(raw) > length {
		builder.WriteString(raw[:length-3])
		builder.WriteString("...")
	} else if orientation == 0 {
		builder.WriteString(strings.Repeat(" ", spacing))
		builder.WriteString(raw)
		builder.WriteString(strings.Repeat(" ", length-len(raw)-spacing))
	} else if orientation == 1 {
		// middle ignores spacing
		left := (length - len(raw)) / 2
		right := length - len(raw) - left
		builder.WriteString(strings.Repeat(" ", left))
		builder.WriteString(raw)
		builder.WriteString(strings.Repeat(" ", right))
	} else if orientation == 2 {
		builder.WriteString(strings.Repeat(" ", length-len(raw)-spacing))
		builder.WriteString(raw)
		builder.WriteString(strings.Repeat(" ", spacing))
	}
	if border {
		builder.WriteString("|")
	}
	builder.WriteString("\n")
	return builder.String()
}
