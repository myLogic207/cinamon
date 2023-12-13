package models

import (
	"database/sql"
	"time"
)

type Key interface {
	// GetID returns the key's ID.
	GetID() uint
	// GetIdentifier returns the key's identifier.
	GetIdentifier() string
	// GetKey returns the key's key.
	GetKey() string
	// GetCreatedAt returns the key's creation time.
	GetCreatedAt() time.Time
	// GetUpdatedAt returns the key's last update time.
	GetUpdatedAt() time.Time
	// IsDeleted returns true if the key is deleted.
	IsDeleted() bool
}

type KeyImpl struct {
	ID         uint
	Identifier string
	Key        string
	created_at time.Time
	updated_at time.Time
	deleted_at sql.NullTime
}

func NewKey(identifier, key string) Key {
	return &KeyImpl{
		Identifier: identifier,
		Key:        key,
	}
}

func (k *KeyImpl) GetID() uint {
	return k.ID
}

func (k *KeyImpl) GetIdentifier() string {
	return k.Identifier
}

func (k *KeyImpl) GetKey() string {
	return k.Key
}

func (k *KeyImpl) GetCreatedAt() time.Time {
	return k.created_at
}

func (k *KeyImpl) GetUpdatedAt() time.Time {
	return k.updated_at
}

func (k *KeyImpl) IsDeleted() bool {
	return k.deleted_at.Valid
}
