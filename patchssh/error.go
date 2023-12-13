package patchssh

import (
	"errors"
	"fmt"
)

var (
	ErrWorkerPoolInit        = errors.New("error initializing worker pool")
	ErrWorkerPoolAlreadyInit = errors.New("worker pool already initialized")
	ErrMissingDBConn         = errors.New("missing database connection")
	ErrSSHConfig             = errors.New("error loading ssh config")
)

type ErrSSHConfigReason struct {
	reason error
}

func (e ErrSSHConfigReason) Error() string {
	return fmt.Sprintf("error loading ssh config: %s", e.reason.Error())
}

func (e ErrSSHConfigReason) Unwrap() error {
	return ErrSSHConfig
}

type ErrInitWorkerPoolReason struct {
	reason error
}

func (e ErrInitWorkerPoolReason) Error() string {
	return fmt.Sprintf("error initializing worker pool: %s", e.reason.Error())
}

func (e ErrInitWorkerPoolReason) Unwrap() error {
	return ErrWorkerPoolInit
}
