package models

import (
	"database/sql"
	"fmt"
	"time"
)

type User interface {
	// GetID returns the user's ID.
	GetID() uint
	// GetUsername returns the user's username.
	GetUsername() string
	// GetNickname returns the user's nickname.
	GetNickname() string
	// GetEmail returns the user's email.
	GetEmail() string
	// GetCreatedAt returns the user's creation time.
	GetCreatedAt() time.Time
	// GetUpdatedAt returns the user's last update time.
	GetUpdatedAt() time.Time
	// IsDeleted returns true if the user is deleted.
	IsDeleted() bool
	// String returns a string representation of the user.
	String() string
}

type UserImpl struct {
	ID         uint
	Username   string
	Nickname   sql.NullString
	Email      string
	created_at time.Time
	updated_at time.Time
	deleted_at sql.NullTime
}

func NewUser(username, nickname string, email string) *UserImpl {
	return &UserImpl{
		Username: username,
		Nickname: sql.NullString{String: nickname, Valid: true},
		Email:    email,
	}
}

func (u *UserImpl) String() string {
	var buf string
	// copy value of pointer to buf
	buf += fmt.Sprintf("ID: %d\t", u.ID)
	buf += "Username: " + u.Username + "\t"
	if u.Nickname.Valid {
		buf += "Nickname: " + u.Nickname.String + "\t"
	}

	// replace last tab with newline
	buf = buf[:len(buf)-1] + "\n"

	return buf
}

func (u *UserImpl) GetID() uint {
	return u.ID
}

func (u *UserImpl) GetUsername() string {
	return u.Username
}

func (u *UserImpl) GetNickname() string {
	if !u.Nickname.Valid {
		return ""
	}
	return u.Nickname.String
}

func (u *UserImpl) GetEmail() string {
	return u.Email
}

func (u *UserImpl) GetCreatedAt() time.Time {
	return u.created_at
}

func (u *UserImpl) GetUpdatedAt() time.Time {
	return u.updated_at
}

func (u *UserImpl) IsDeleted() bool {
	// no check because if field is valid, user had to be deleted
	return u.deleted_at.Valid
}
