package models

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/myLogic207/cinnamon/internal/dbconnect"
)

type UserDB interface {
	// Register registers a new user in the database.
	Register(ctx context.Context, user User, password string) error
	// GetAll retrieves all users from the database.
	GetAll(ctx context.Context) ([]User, error)
	// GetUser retrieves a user from the database by its ID.
	GetUser(ctx context.Context, where map[string]interface{}, limit int) (User, error)
	// GetByUsername retrieves a user from the database by its username.
	GetByUsername(ctx context.Context, username string) (User, error)
	// GetByEmail retrieves a user from the database by its email.
	GetByEmail(ctx context.Context, email string) (User, error)
	// Update updates a user in the database.
	Update(ctx context.Context, user User) error
	// UpdatePassword updates a user's password in the database.
	UpdatePassword(ctx context.Context, user User, password string) error
	// DeleteUser deletes a user from the database.
	DeleteUser(ctx context.Context, id uint) error
	// Authenticate authenticates a user by its username and password.
	Authenticate(ctx context.Context, username, password string) (User, error)
}

const (
	user_TABLENAME         = "users"
	userPassword_TABLENAME = "hashes"
)

var (
	ErrUserNotFound      = errors.New("no user found")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrRegisteringUser   = errors.New("error registering user")
)

type UserDBImpl struct {
	*dbconnect.DB
}

func NewUserDB(db *dbconnect.DB) (UserDB, error) {
	return &UserDBImpl{db}, nil
}

func (db *UserDBImpl) Register(ctx context.Context, user User, passwordHash string) error {
	return db.Transaction(ctx, func(tx *sql.Tx) error {
		// check if user already exists
		var id uint
		query, args, err := db.NewBuilder().
			Select("id").
			From(user_TABLENAME).
			Where(squirrel.Eq{"username": user.GetUsername()}).ToSql()
		if err != nil {
			return err
		}
		if err := tx.QueryRow(query, args...).Scan(&id); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}

		// create new user
		res, err := db.NewBuilder().
			Insert(user_TABLENAME).
			Columns("username", "nickname", "email").
			Values(user.GetUsername(), user.GetUsername(), user.GetEmail()).
			RunWith(tx).Exec()
		if err != nil {
			return err
		} else if rows, err := res.RowsAffected(); err != nil {
			return err
		} else if rows != 1 {
			return ErrRegisteringUser
		}
		// get user id
		var userid uint
		err = db.NewBuilder().
			Select("id").
			From(user_TABLENAME).
			Where(squirrel.Eq{"username": user.GetUsername()}).
			RunWith(tx).QueryRow().Scan(&userid)
		if err != nil {
			return err
		}

		// create new user password
		res, err = db.NewBuilder().
			Insert(userPassword_TABLENAME).
			Columns("user_id", "pw_hash").
			Values(userid, passwordHash).
			RunWith(tx).Exec()
		if err != nil {
			return err
		} else if rows, err := res.RowsAffected(); err != nil {
			return err
		} else if rows != 1 {
			return ErrRegisteringUser
		} else {
			return nil
		}
	}, &sql.TxOptions{
		ReadOnly: false,
	})
}

func (db *UserDBImpl) GetAll(ctx context.Context) (users []User, err error) {
	users = []User{}
	err = db.Transaction(ctx, func(tx *sql.Tx) error {
		rawUser, err := db.NewBuilder().Select("id, username, nickname, email, created_at").From(user_TABLENAME).RunWith(tx).Query()
		if err != nil {
			return err
		}
		defer rawUser.Close()
		for rawUser.Next() {
			id := uint(0)
			username := ""
			nickname := sql.NullString{}
			email := ""
			created_at := time.Time{}
			deleted_at := sql.NullTime{}
			if err := rawUser.Scan(&id, &username, &nickname, &email, &created_at, &deleted_at); err != nil {
				return err
			}
			user := &UserImpl{
				ID:         id,
				Username:   username,
				Nickname:   nickname,
				Email:      email,
				created_at: created_at,
				deleted_at: deleted_at,
			}
			users = append(users, user)
		}
		return nil
	}, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
		ReadOnly:  true,
	})
	return
}

func (db *UserDBImpl) GetUser(ctx context.Context, where map[string]interface{}, limit int) (user User, err error) {
	err = db.Transaction(ctx, func(tx *sql.Tx) error {
		// get user
		id := uint(0)
		username := ""
		nickname := sql.NullString{}
		email := ""
		created_at := time.Time{}
		deleted_at := sql.NullTime{}

		err := db.NewBuilder().Select("id, username, nickname, email, created_at").From(user_TABLENAME).Where(where).Limit(uint64(limit)).RunWith(tx).QueryRow().Scan()
		if err != nil {
			return err
		}

		user = &UserImpl{
			ID:         id,
			Username:   username,
			Nickname:   nickname,
			Email:      email,
			created_at: created_at,
			deleted_at: deleted_at,
		}

		return nil
	}, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
		ReadOnly:  true,
	})
	return
}

func (db *UserDBImpl) GetByUsername(ctx context.Context, username string) (user User, err error) {
	user = &UserImpl{}
	err = db.Transaction(ctx, func(tx *sql.Tx) error {
		return nil
	}, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
		ReadOnly:  true,
	})
	return
}

func (db *UserDBImpl) GetByEmail(ctx context.Context, email string) (user User, err error) {
	user = &UserImpl{}
	err = db.Transaction(ctx, func(tx *sql.Tx) error {
		return nil
	}, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
		ReadOnly:  true,
	})
	return
}

func (db *UserDBImpl) Update(ctx context.Context, user User) (err error) {
	err = db.Transaction(ctx, func(tx *sql.Tx) error {
		return nil
	}, &sql.TxOptions{})
	return
}

func (db *UserDBImpl) UpdatePassword(ctx context.Context, user User, passwordHash string) error {
	return db.Transaction(ctx, func(tx *sql.Tx) error {
		result, err := db.NewBuilder().
			Update(userPassword_TABLENAME).
			Set("pw_hash", passwordHash).
			Where(squirrel.Eq{"user_id": user.GetID()}).
			RunWith(tx).Exec()
		if err != nil {
			return err
		}

		if rows, err := result.RowsAffected(); err != nil {
			return err
		} else if rows != 1 {
			return errors.New("could not update user password")
		}

		return nil
	}, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
		ReadOnly:  false,
	})
}

func (db *UserDBImpl) DeleteUser(ctx context.Context, id uint) error {
	err := db.Transaction(ctx, func(tx *sql.Tx) error {
		return nil
	}, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
		ReadOnly:  false,
	})
	return err
}

func (db *UserDBImpl) Authenticate(ctx context.Context, username, password string) (User, error) {
	user := &UserImpl{}
	passwordHash := ""

	err := db.Transaction(ctx, func(tx *sql.Tx) error {
		// get password hash
		err := db.NewBuilder().
			// Select("*").
			Select("pw_hash").
			From(userPassword_TABLENAME).
			Join("users ON users.id = hashes.user_id").
			Where(squirrel.Eq{"username": username}).
			RunWith(tx).QueryRow().Scan(&passwordHash)
		if err != nil {
			return err
		}

		if !CheckPasswordHash(password, passwordHash) {
			return ErrInvalidPassword
		}

		// Update last login time
		result, err := db.NewBuilder().
			Update(user_TABLENAME).
			Set("last_login", time.Now().UTC()).
			Where(squirrel.Eq{"username": username}).
			RunWith(tx).Exec()
		if err != nil {
			return err
		} else if rows, err := result.RowsAffected(); err != nil {
			return err
		} else if rows != 1 {
			return errors.New("could not update user last login time")
		}

		return nil
	}, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
		ReadOnly:  false,
	})

	return user, err
}
