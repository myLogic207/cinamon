package patchssh

import (
	"context"
	"errors"
	"testing"

	"github.com/myLogic207/gotils/config"
	log "github.com/myLogic207/gotils/logger"
)

var (
	TESTSHELL *ShellWrapper = nil
)

func TestMain(m *testing.M) {
	testlogger, err := log.NewLogger(config.NewConfigWithInitialValues(map[string]interface{}{
		"PREFIX":       "TEST",
		"PREFIXLENGTH": 8,
	}))
	if err != nil {
		panic(err)
	}
	TESTSHELL = NewShellWrapper(testlogger)
	m.Run()
}

func TestStdout(t *testing.T) {
	// Test that creating a new key works without errors
	t.Log("Executing echo")
	out, err := TESTSHELL.Execute(context.TODO(), "echo test")
	if err != nil {
		t.Errorf("Error executing echo: %v", err)
		t.FailNow()
	}
	t.Logf("Output: %s", out)
}

func TestStderr(t *testing.T) {
	// Test that creating a new key works without errors
	_, err := TESTSHELL.Execute(context.TODO(), "asdiauhdfasuiodh")
	if err == nil || !errors.Is(err, ErrCommandNotFound) {
		t.Errorf("Error executing echo: %v", err)
		t.FailNow()
	}
}
