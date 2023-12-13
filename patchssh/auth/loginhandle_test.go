package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/myLogic207/cinnamon/internal/dbconnect"
	"github.com/myLogic207/cinnamon/internal/models"
	"github.com/myLogic207/gotils/config"
	"golang.org/x/crypto/ssh"
)

type TestConnMetadata struct {
	user string
}

func (t TestConnMetadata) User() string {
	return t.user
}

func (t TestConnMetadata) SessionID() []byte {
	return []byte{}
}

func (t TestConnMetadata) ClientVersion() []byte {
	return []byte{}
}

func (t TestConnMetadata) ServerVersion() []byte {
	return []byte{}
}

func (t TestConnMetadata) RemoteAddr() net.Addr {
	return &net.IPAddr{
		IP: net.IPv4(127, 0, 0, 1),
	}
}

func (t TestConnMetadata) LocalAddr() net.Addr {
	return nil
}

var defaultOptions = map[string]interface{}{
	"Logger": map[string]interface{}{
		"LEVEL":  "DEBUG",
		"PREFIX": "TEST-DB",
		"WRITERS": map[string]interface{}{
			"STDOUT": true,
			"FILE": map[string]interface{}{
				"ACTIVE": false,
			},
		},
	},
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
}

func TestPublicKeyCallback(t *testing.T) {
	testCtx := context.Background()
	options := config.NewWithInitialValues(defaultOptions)

	db, mock, err := dbconnect.NewDBMock(options)
	if err != nil {
		t.Fatalf("Failed to create mock db: %v", err)
	}
	keydb, err := models.NewKeyDB(db)
	if err != nil {
		t.Fatalf("Failed to create key db: %v", err)
	}
	manager := NewAuthManager(keydb)

	// Test known user with supported key type
	testPubKey, _, _ := ed25519.GenerateKey(rand.Reader)
	key, _ := ssh.NewPublicKey(testPubKey)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT keystring FROM sshkeys WHERE identifier = ?").WithArgs("known").WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()
	// test before add aka unknown key
	if perms, err := manager.PublicKeyCallback(TestConnMetadata{user: "known"}, key); err == nil {
		t.Errorf("Expected error of type ErrAuthFailedReason, got %v", err)
	} else if perms != nil {
		t.Errorf("Expected nil permissions, got %v", perms)
	}

	pubKey := strings.Trim(string(ssh.MarshalAuthorizedKey(key)), "\n")
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO sshkeys").WithArgs("known", pubKey).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	// add key
	if err := manager.AddKnownHost(testCtx, "known", key); err != nil {
		t.Fatalf("Failed to add known host: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT keystring FROM sshkeys WHERE identifier = ?").WithArgs("known").WillReturnRows(sqlmock.NewRows([]string{"keystring"}).AddRow(string(ssh.MarshalAuthorizedKey(key))))
	mock.ExpectCommit()
	// test after add aka known key
	if perms, err := manager.PublicKeyCallback(TestConnMetadata{user: "known"}, key); err != nil {
		t.Errorf("Expected nil error, got %v", err)
	} else if perms == nil {
		t.Errorf("Expected non-nil permissions, got %v", perms)
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT keystring FROM sshkeys WHERE identifier = ?").WithArgs("unknown").WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()
	// Test unknown user
	_, err = manager.PublicKeyCallback(TestConnMetadata{user: "unknown"}, key)
	if err == nil || !errors.Is(err, ErrAuthFailed) {
		t.Errorf("Expected error of type ErrAuthFailedReason, got %v", err)
	}
}
