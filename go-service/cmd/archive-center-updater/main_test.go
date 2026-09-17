package main

import "testing"

func TestRunApplyPendingNoPending(t *testing.T) {
	if code := run([]string{"apply-pending", "--root", t.TempDir()}); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	if code := run([]string{"unknown", "--root", t.TempDir()}); code == 0 {
		t.Fatal("unknown command returned success")
	}
}
