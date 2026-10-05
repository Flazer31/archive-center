package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestHUDStreamOwnerLifecycleHTTP(t *testing.T) {
	node := os.Getenv("ARCHIVE_CENTER_NODE_BINARY")
	if node == "" {
		node = "node"
	}
	cmd := exec.Command(node, filepath.Join("..", "..", "..", "ops", "hud-stream-lifecycle-test.cjs"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("production HUD owners with isolated HTTP: %v\n%s", err, out)
	}
}
