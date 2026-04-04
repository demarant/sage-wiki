package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestIsAvailable(t *testing.T) {
	// Git should be available in CI/dev environments
	if !IsAvailable() {
		t.Skip("git not available")
	}
}

func TestInitAndIsRepo(t *testing.T) {
	if !IsAvailable() {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	if IsRepo(dir) {
		t.Error("should not be a repo yet")
	}

	if err := Init(dir); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if !IsRepo(dir) {
		t.Error("should be a repo after init")
	}
}

func TestAutoCommit(t *testing.T) {
	if !IsAvailable() {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	Init(dir)

	// Configure git user for commit
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")

	// Create a file and commit
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0644)

	if err := AutoCommit(dir, "test commit"); err != nil {
		t.Fatalf("AutoCommit: %v", err)
	}

	hash, msg, err := LastCommit(dir)
	if err != nil {
		t.Fatalf("LastCommit: %v", err)
	}
	if hash == "" {
		t.Error("expected commit hash")
	}
	if msg != "test commit" {
		t.Errorf("expected 'test commit', got %q", msg)
	}
}

func TestStatus(t *testing.T) {
	if !IsAvailable() {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	Init(dir)

	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new"), 0644)

	status, err := Status(dir)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status == "" {
		t.Error("expected non-empty status with untracked file")
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := append([]string{"-C", dir}, args...)
	out, err := execGit(cmd...)
	if err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
}

func execGit(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
