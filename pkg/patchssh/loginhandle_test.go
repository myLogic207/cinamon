package patchssh

import (
	"os"
	"testing"
)

func TestKeyGeneration(t *testing.T) {
	// Test that creating a new key works without errors
	t.Log("Creating new key")
	key, err := ensureKeyFile("test.key")
	if err != nil {
		t.Errorf("Error creating a new key: %v", err)
		t.FailNow()
	}
	t.Logf("Key: %v", key)
	t.Cleanup(func() {
		t.Log("Cleaning up")
		if err := os.Remove("test.key"); err != nil {
			t.Errorf("Error removing test.key: %v", err)
		}
		if err := os.Remove("test.key.pub"); err != nil {
			t.Errorf("Error removing test.key: %v", err)
		}
	})
}

func TestReadKnownHosts(t *testing.T) {
	// Test that creating a new key works without errors
	t.Log("Creating new key")
	publicKey := []byte("ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQC1Uj1c8MZhvoQKVckJuopU0Z09ec2auHPExX9HRKGgF3pZU/aeIN31wXFEqWA1IEzSbUNEFmsGRoEovGyYwNC2vVeldmf+/DENOst7tKaiD7VTRp0+1+9gZNZYtZFUgDSiMxBTz5X79eZkmKRwxAdFVUnnvL5L7HQglEqjJ3cvdQ==")
	if err := os.WriteFile("test.known_hosts", publicKey, 0644); err != nil {
		t.Errorf("Error writing test.known_hosts: %v", err)
		t.FailNow()
	}

	knownKeys, err := loadKnownKeys("test.known_hosts")
	if err != nil {
		t.Errorf("Error reading file: %v", err)
		t.FailNow()
	}
	t.Logf("Known Keys: %#v", knownKeys)
	t.Cleanup(func() {
		t.Log("Cleaning up")
		if err := os.Remove("test.known_hosts"); err != nil {
			t.Errorf("Error removing test.known_hosts: %v", err)
		}
	})
}
