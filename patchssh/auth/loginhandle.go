// Package auth provides authentication management for SSH connections.
package auth

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/myLogic207/cinnamon/internal/models"
	"golang.org/x/crypto/ssh"
)

// SupportedKeyTypes is the list of supported SSH key types.
var SupportedKeyTypes = []string{"ssh-ed25519"}

// ErrKeyNotSupported indicates that the key type is not supported.
var ErrKeyNotSupported = errors.New("key type not supported")

// ErrAuthFailed indicates a generic authentication failure.
var ErrAuthFailed = errors.New("authentication failed")

// ErrAuthFailedReason provides additional context for authentication failure.
type ErrAuthFailedReason struct {
	reason error
}

// Error returns the formatted error message.
func (e ErrAuthFailedReason) Error() string {
	return fmt.Sprintf("authentication failed: %s", e.reason.Error())
}

// Unwrap returns the underlying error.
func (e ErrAuthFailedReason) Unwrap() error {
	return ErrAuthFailed
}

// AuthManager manages SSH authentication.
type AuthManager struct {
	models.KeyDB
}

// NewAuthManager creates a new AuthManager instance.
func NewAuthManager(keyDB models.KeyDB) *AuthManager {
	return &AuthManager{keyDB}
}

// guestLogin returns guest user permissions if the user is "guest".
func (km *AuthManager) guestLogin(user string) *ssh.Permissions {
	if user != "guest" {
		return nil
	}
	return &ssh.Permissions{
		Extensions: map[string]string{
			"pubkey-fp": "guest",
		},
	}
}

// PublicKeyCallback handles public key authentication.
func (km *AuthManager) PublicKeyCallback(c ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
	if guest := km.guestLogin(c.User()); guest != nil {
		return guest, nil
	}

	if !slices.Contains(SupportedKeyTypes, pubKey.Type()) {
		return nil, ErrKeyNotSupported
	}

	if ok, err := km.CheckKnownHost(context.Background(), c.User(), pubKey); err != nil {
		return nil, ErrAuthFailedReason{err}
	} else if !ok {
		return nil, ErrAuthFailed
	}

	return &ssh.Permissions{
		CriticalOptions: map[string]string{
			"pubkey-fp": ssh.FingerprintSHA256(pubKey),
		},
		Extensions: map[string]string{
			"permit-X11-forwarding":   "true",
			"permit-agent-forwarding": "true",
		},
	}, nil
}

// PasswordAuth handles password authentication.
func (km *AuthManager) PasswordAuth(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	// Ignore password for guest user.
	if guest := km.guestLogin(conn.User()); guest != nil {
		return guest, nil
	}

	return nil, ErrAuthFailedReason{errors.New("password authentication not supported")}
}

// NoAuthCallback handles scenarios where no authentication method is supported.
func (km *AuthManager) NoAuthCallback(conn ssh.ConnMetadata) (*ssh.Permissions, error) {
	// Allowing guest user access.
	if guest := km.guestLogin(conn.User()); guest != nil {
		return guest, nil
	}

	return nil, ErrAuthFailedReason{errors.New("no authentication not method supported")}
}

// KeyboardInteractiveAuth handles keyboard-interactive authentication.
// Uncomment and implement if needed in the future.
/*
func (km *AuthManager) KeyboardInteractiveAuth(conn ssh.ConnMetadata, challenge ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
	// Implementation goes here.
}
*/
