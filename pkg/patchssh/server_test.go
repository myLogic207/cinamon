package patchssh

import (
	"context"
	"gotils/config"
	"testing"
	"time"
)

func TestNewServerTCPSocket(t *testing.T) {
	// Test that creating a new socket works without errors
	testCtx, cancel := context.WithCancel(context.Background())
	t.Log("Creating new config")
	cnf := config.NewConfigWithInitialValues(map[string]interface{}{
		"ADDRESS": "127.0.0.1",
		"PORT":    8080,
		"WORKERS": 3,
	})
	t.Log("Creating new socket")
	socket, err := NewServer(cnf)
	if err != nil {
		t.Errorf("Error creating a new socket: %v", err)
		t.FailNow()
	}

	if err := socket.Serve(testCtx); err != nil {
		t.Log(err)
		t.FailNow()
	}

	t.Log("Server started, stopping it")
	// wait one second to make sure the socket is listening
	time.Sleep(1 * time.Millisecond)
	cancel()
	// wait one second to make sure the socket is closed
	time.Sleep(1 * time.Millisecond)

	// Test that creating a new socket works without errors
	testCtx, cancel = context.WithCancel(context.Background())
	if err := socket.Serve(testCtx); err != nil {
		t.Log(err)
		t.FailNow()
	}

	// wait one second to make sure the socket is listening
	time.Sleep(1 * time.Millisecond)
	cancel()
	// wait one second to make sure the socket is closed
	time.Sleep(1 * time.Millisecond)
}
