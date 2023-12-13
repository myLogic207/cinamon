package models

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/myLogic207/cinnamon/internal/dbconnect"
	"golang.org/x/crypto/ssh"
)

// tablespec:
// Tablename: keys
// Columns:
// 		id: INTEGER PRIMARY KEY
// 		identifier: TEXT NOT NULL UNIQUE
// 		key: TEXT NOT NULL
// 		created_at: TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
// 		updated_at: TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
// 		deleted_at: TIMESTAMP

type KeyDB interface {
	// SetHostKey sets the private key for the host
	SetHostKey(ctx context.Context, pemBytes []byte) error
	// returns the private key for the host, pem encoded
	GetHostKey(ctx context.Context) (pemBytes []byte, err error)
	// adds a new known host to the database
	AddKnownHost(ctx context.Context, hostIdentifier string, key ssh.PublicKey) error
	// checks if the given host is known
	CheckKnownHost(ctx context.Context, hostIdentifier string, key ssh.PublicKey) (bool, error)
}

const key_TABLENAME = "sshkeys"

var (
	ErrKeyNotFound           = errors.New("no key found")
	ErrInvalidHostIdentifier = errors.New("invalid host identifier")
	ErrHostAlreadyKnown      = errors.New("host already known")
	ErrTableNotFound         = errors.New("table not found")
)

type KeyDBImpl struct {
	*dbconnect.DB
}

func NewKeyDB(db *dbconnect.DB) (KeyDB, error) {
	return &KeyDBImpl{db}, nil
}

func (db *KeyDBImpl) InitKeyTable(ctx context.Context, privatePem string) error {
	exists, err := db.CheckTableExists(key_TABLENAME)
	if err != nil {
		return err
	} else if !exists {
		return ErrTableNotFound
	}

	db.Transaction(ctx, func(tx *sql.Tx) error {
		// check if localhost key is already set, if not, set it
		query, args, txErr := db.NewBuilder().Select("keystring").From(key_TABLENAME).Where(squirrel.Eq{"identifier": "localhost"}).ToSql()
		if txErr != nil {
			return txErr
		}
		var key string
		if err := tx.QueryRow(query, args...).Scan(&key); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		} else if errors.Is(err, sql.ErrNoRows) {
			// localhost key not set, set it
			result, txErr := db.NewBuilder().Insert(key_TABLENAME).Columns("identifier", "keystring").Values("localhost", privatePem).RunWith(tx).Exec()
			if txErr != nil {
				return txErr
			}
			if rows, txErr := result.RowsAffected(); txErr != nil {
				return txErr
			} else if rows != 1 {
				return errors.New("could not insert localhost key")
			} else {
				return nil
			}
		}

		return nil
	}, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
		ReadOnly:  false,
	})
	return nil
}

func (db *KeyDBImpl) SetHostKey(ctx context.Context, pemBytes []byte) error {
	return db.Transaction(ctx, func(tx *sql.Tx) error {
		var keystring string
		// check if localhost key is already set, if not, set it
		err := db.NewBuilder().
			Select("keystring").
			From(key_TABLENAME).
			Where(squirrel.Eq{"identifier": "localhost"}).
			RunWith(tx).QueryRow().Scan(&keystring)

		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		} else if errors.Is(err, sql.ErrNoRows) {
			// localhost key not set, set it
			res, err := db.NewBuilder().Insert(key_TABLENAME).Columns("identifier", "keystring").Values("localhost", string(pemBytes)).RunWith(tx).Exec()
			if err != nil {
				return err
			} else if rows, err := res.RowsAffected(); err != nil {
				return err
			} else if rows != 1 {
				return errors.New("could not insert localhost key")
			} else {
				return nil
			}
		}

		res, err := db.NewBuilder().Update(key_TABLENAME).Set("keystring", string(pemBytes)).Where(squirrel.Eq{"identifier": "localhost"}).RunWith(tx).Exec()
		if err != nil {
			return err
		} else if rows, err := res.RowsAffected(); err != nil {
			return err
		} else if rows != 1 {
			return errors.New("could not update localhost key")
		} else {
			return nil
		}
	}, &sql.TxOptions{
		ReadOnly: false,
	})
}

func (db *KeyDBImpl) GetHostKey(ctx context.Context) (pemString []byte, err error) {
	db.Transaction(ctx, func(tx *sql.Tx) error {
		pemString = []byte{}
		err = db.NewBuilder().
			Select("keystring").
			From(key_TABLENAME).
			Where(squirrel.Eq{"identifier": "localhost"}).
			RunWith(tx).QueryRow().Scan(&pemString)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		} else if errors.Is(err, sql.ErrNoRows) {
			err = ErrKeyNotFound
			return nil
		}
		return nil
	}, &sql.TxOptions{
		ReadOnly:  true,
		Isolation: sql.LevelReadCommitted,
	})
	return []byte(pemString), nil
}

func (db *KeyDBImpl) AddKnownHost(ctx context.Context, hostIdentifier string, key ssh.PublicKey) (err error) {
	keyString := strings.Trim(string(ssh.MarshalAuthorizedKey(key)), "\n")
	return db.Transaction(ctx, func(tx *sql.Tx) error {
		res, err := db.NewBuilder().
			Insert(key_TABLENAME).
			Columns("identifier", "keystring").
			Values(hostIdentifier, keyString).
			RunWith(tx).Exec()
		if err != nil {
			return err
		} else if rows, err := res.RowsAffected(); err != nil {
			return err
		} else if rows != 1 {
			return errors.New("could not insert key")
		}

		return nil
	}, &sql.TxOptions{
		ReadOnly: false,
	})
}

func (db *KeyDBImpl) CheckKnownHost(ctx context.Context, hostIdentifier string, key ssh.PublicKey) (ok bool, err error) {
	db.Transaction(ctx, func(tx *sql.Tx) error {
		var keyString string

		err = db.NewBuilder().
			Select("keystring").
			From(key_TABLENAME).
			Where(squirrel.Eq{"identifier": hostIdentifier}).
			RunWith(tx).QueryRow().Scan(&keyString)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		} else if errors.Is(err, sql.ErrNoRows) {
			err = ErrKeyNotFound
			return nil
		}
		parsedKeyString, _, _, _, parseErr := ssh.ParseAuthorizedKey([]byte(keyString))
		if parseErr != nil {
			err = parseErr
			return nil
		}
		ok = comparePublickeys(key, parsedKeyString)
		return nil
	}, &sql.TxOptions{
		ReadOnly:  true,
		Isolation: sql.LevelReadCommitted,
	})
	return
}

func comparePublickeys(key1, key2 ssh.PublicKey) bool {
	return bytes.Equal(ssh.MarshalAuthorizedKey(key1), ssh.MarshalAuthorizedKey(key2))
}
