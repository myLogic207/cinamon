package models

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/myLogic207/cinnamon/internal/dbconnect"
	"github.com/myLogic207/gotils/config"
)

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

func TestUserAuth(t *testing.T) {
	options := config.NewWithInitialValues(defaultOptions)
	db, mock, err := dbconnect.NewDBMock(options)
	if err != nil {
		t.Fatal(err)
	}
	userDB, err := NewUserDB(db)
	if err != nil {
		panic(err)
	}

	testCtx := context.Background()
	username := "testuser"
	useremail := "tesuser@example.net"
	password := "testpassword"
	passwordHash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM users WHERE username = ?").WithArgs(username).WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO users").WithArgs(username, username, useremail).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM users WHERE username = ?").WithArgs(username).WillReturnRows(sqlmock.NewRows([]string{"ID"}).AddRow(1))
	mock.ExpectExec("INSERT INTO hashes").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := userDB.Register(testCtx, NewUser(username, username, useremail), passwordHash); err != nil {
		t.Fatal(err)
	}

	// queryUserPass := "SELECT \\* FROM hashes"
	queryUserPass := "SELECT pw_hash FROM hashes JOIN users ON users.id = hashes.user_id WHERE username = ?"
	hashrow := sqlmock.NewRows([]string{"pw_hash"}).AddRow(passwordHash)
	mock.ExpectBegin()
	mock.ExpectQuery(queryUserPass).WithArgs(username).WillReturnRows(hashrow)
	mock.ExpectExec("UPDATE users").WithArgs(sqlmock.AnyArg(), username).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if _, err := userDB.Authenticate(testCtx, username, password); err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	mock.ExpectQuery(queryUserPass).WithArgs(sqlmock.AnyArg()).WillReturnRows(hashrow)
	mock.ExpectRollback()
	if _, err := userDB.Authenticate(testCtx, username, "wrongpassword"); err == nil {
		t.Fatal("expected error")
	}

	mock.ExpectBegin()
	mock.ExpectQuery(queryUserPass).WithArgs(sqlmock.AnyArg()).WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()
	if _, err := userDB.Authenticate(testCtx, "wronguser", passwordHash); err == nil {
		t.Fatal("expected error")
	}
}

func TestUpdatePassword(t *testing.T) {
	options := config.NewWithInitialValues(defaultOptions)
	db, mock, err := dbconnect.NewDBMock(options)
	if err != nil {
		t.Fatal(err)
	}
	userDB, err := NewUserDB(db)
	if err != nil {
		panic(err)
	}

	testCtx := context.Background()
	username := "testuser"
	useremail := "test@example.net"
	password := "testpassword"
	passwordHash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM users WHERE username = ?").WithArgs(username).WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO users").WithArgs(username, username, useremail).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT id FROM users WHERE username = ?").WithArgs(username).WillReturnRows(sqlmock.NewRows([]string{"ID"}).AddRow(1))
	mock.ExpectExec("INSERT INTO hashes").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := userDB.Register(testCtx, NewUser(username, username, useremail), passwordHash); err != nil {
		t.Fatal(err)
	}

	// check login with old password
	queryUserPass := "SELECT pw_hash FROM hashes JOIN users ON users.id = hashes.user_id WHERE username = ?"
	hashrow := sqlmock.NewRows([]string{"pw_hash"}).AddRow(passwordHash)

	mock.ExpectBegin()
	mock.ExpectQuery(queryUserPass).WithArgs(username).WillReturnRows(hashrow)
	mock.ExpectExec("UPDATE users").WithArgs(sqlmock.AnyArg(), username).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if _, err := userDB.Authenticate(testCtx, username, password); err != nil {
		t.Fatal(err)
	}

	newPassword := "newpassword"
	newPasswordHash, err := HashPassword(newPassword)
	if err != nil {
		t.Fatal(err)
	}
	newHashrow := sqlmock.NewRows([]string{"pw_hash"}).AddRow(newPasswordHash)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE hashes").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := userDB.UpdatePassword(testCtx, NewUser(username, username, useremail), newPasswordHash); err != nil {
		t.Fatal(err)
	}

	// check login with new password
	mock.ExpectBegin()
	mock.ExpectQuery(queryUserPass).WithArgs(username).WillReturnRows(newHashrow)
	mock.ExpectExec("UPDATE users").WithArgs(sqlmock.AnyArg(), username).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if _, err := userDB.Authenticate(testCtx, username, newPassword); err != nil {
		t.Fatal(err)
	}

}
