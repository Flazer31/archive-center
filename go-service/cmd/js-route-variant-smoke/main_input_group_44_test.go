package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func Test44InputGroupHostLifecycleReplay(t *testing.T) {
	node := os.Getenv("ARCHIVE_CENTER_NODE_BINARY")
	if node == "" {
		var err error
		node, err = exec.LookPath("node")
		if err != nil {
			t.Fatal(err)
		}
	}
	script, err := filepath.Abs("../../../ops/input-group-smoke.cjs")
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, script).CombinedOutput()
	if err != nil {
		t.Fatalf("production owner replay: %v\n%s", err, out)
	}
	t.Log(string(out))
}
