package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCompleteTurnDispatchCarriesCurrentConfigWithoutPersistingSecrets(t *testing.T) {
	node := os.Getenv("ARCHIVE_CENTER_NODE_BINARY")
	if node == "" {
		node = "node"
	}
	cmd := exec.Command(node, filepath.Join("..", "..", "..", "ops", "critic-config-delivery-test.cjs"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("production complete-turn config delivery: %v\n%s", err, out)
	}
}
