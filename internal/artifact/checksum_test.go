package artifact

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSHA256FileReturnsLowercaseHexDigest(t *testing.T) {
	path := filepath.Join(t.TempDir(), "artifact.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	digest, err := SHA256File(path)
	if err != nil {
		t.Fatalf("SHA256File returned error: %v", err)
	}

	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if digest != want {
		t.Fatalf("SHA256File() = %q, want %q", digest, want)
	}
}

func TestSHA256FileMissingPathReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.txt")
	if _, err := SHA256File(path); err == nil {
		t.Fatal("SHA256File returned nil error for missing path")
	}
}
