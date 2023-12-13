package patchssh

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/myLogic207/cinnamon/internal/dbconnect"
	"github.com/myLogic207/cinnamon/internal/models"
	"github.com/myLogic207/gotils/config"
	"golang.org/x/crypto/ssh"
)

var TESTSERVER *SocketServer
var TESTCLIENTCONFIG *ssh.ClientConfig

var keyDBconf = map[string]interface{}{
	"DB": map[string]interface{}{
		"TYPE": "sqlite3",
	},
	"POOL": map[string]interface{}{
		"CONNS_OPEN":    3,
		"CONNS_IDLE":    1,
		"MAX_LIFETIME":  0,
		"MAX_IDLE_TIME": 0,
	},
	"CACHE": map[string]interface{}{
		"ACTIVE": false,
	},
	"LOGGER": map[string]interface{}{
		"LEVEL":  "DEBUG",
		"PREFIX": "TEST-DB",
		"WRITERS": map[string]interface{}{
			"STDOUT": true,
			"FILE": map[string]interface{}{
				"ACTIVE": false,
			},
		},
	},
}

var testServerConf = map[string]interface{}{
	"ADDRESS": "127.0.0.1",
	"PORT":    22222,
	"WORKERS": 3,
}

const USERNAME = "testuser"

var dbMock sqlmock.Sqlmock
var pubKey string

func TestMain(m *testing.M) {
	options := config.NewWithInitialValues(keyDBconf)
	db, mock, err := dbconnect.NewDBMock(options)
	if err != nil {
		panic(err)
	}
	dbMock = mock
	kdb, err := models.NewKeyDB(db)
	if err != nil {
		panic(err)
	}
	initServer(kdb)

	sshPubkey := initClient()
	pubKey = strings.Trim(string(ssh.MarshalAuthorizedKey(sshPubkey)), "\n")
	testCtx := context.TODO()
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO sshkeys").WithArgs(USERNAME, pubKey).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := kdb.AddKnownHost(testCtx, USERNAME, sshPubkey); err != nil {
		panic(err)
	}
	m.Run()
}

func initServer(kdb models.KeyDB) {
	_, hostPrivKey, _ := ed25519.GenerateKey(rand.Reader)
	privPemBlock, err := ssh.MarshalPrivateKey(crypto.PrivateKey(hostPrivKey), "test")
	if err != nil {
		panic(err)
	}
	privPemString := string(pem.EncodeToMemory(privPemBlock))
	testServerConf := config.NewWithInitialValues(testServerConf)
	if err := testServerConf.Set("HOSTKEY", privPemString, true); err != nil {
		panic(err)
	}
	testServer, err := NewServer(testServerConf, kdb)
	if err != nil {
		panic(err)
	}
	testCtx := context.TODO()
	dbMock.ExpectBegin()
	dbMock.ExpectQuery("SELECT keystring FROM sshkeys WHERE identifier = ?").WithArgs("localhost").WillReturnError(sql.ErrNoRows)
	dbMock.ExpectCommit()
	dbMock.ExpectBegin()
	dbMock.ExpectQuery("SELECT keystring FROM sshkeys WHERE identifier = ?").WithArgs("localhost").WillReturnError(sql.ErrNoRows)
	dbMock.ExpectExec("INSERT INTO sshkeys").WithArgs("localhost", string(privPemString)).WillReturnResult(sqlmock.NewResult(1, 1))
	dbMock.ExpectCommit()
	if err := testServer.Serve(testCtx); err != nil {
		panic(err)
	}
	TESTSERVER = testServer
}

func initClient() ssh.PublicKey {
	testPublicKey, testPrivateKey, _ := ed25519.GenerateKey(rand.Reader)
	key, err := ssh.MarshalPrivateKey(testPrivateKey, "")
	if err != nil {
		panic(err)
	}
	keyBytes := pem.EncodeToMemory(key)
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		panic(err)
	}

	clientConfig := &ssh.ClientConfig{
		User: USERNAME,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback:   ssh.InsecureIgnoreHostKey(),
		HostKeyAlgorithms: []string{"ssh-ed25519"},
		BannerCallback:    ssh.BannerDisplayStderr(),
	}

	sshPubkey, err := ssh.NewPublicKey(testPublicKey)
	if err != nil {
		panic(err)
	}

	TESTCLIENTCONFIG = clientConfig
	return sshPubkey
}

func TestServerConnect(t *testing.T) {
	dbMock.ExpectBegin()
	dbMock.ExpectQuery("SELECT keystring FROM sshkeys WHERE identifier = ?").WithArgs(USERNAME).WillReturnRows(sqlmock.NewRows([]string{"keystring"}).AddRow(pubKey))
	dbMock.ExpectCommit()

	_, err := ssh.Dial("tcp", "127.0.0.1:22222", TESTCLIENTCONFIG)
	if err != nil {
		panic(err)
	}
}
