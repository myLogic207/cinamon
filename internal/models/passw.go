package models

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

const (
	HASHCOST = 10
)

type UserPassword struct {
	ID     *uint
	UserID uint
	Hash   string
}

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), HASHCOST)
	return string(hash), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

var (
	ErrInvalidPassword = errors.New("invalid password")
)
