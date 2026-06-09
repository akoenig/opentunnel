package artifact

import (
	"runtime"
	"strings"
	"testing"
)

func TestPlatformKeyReturnsOSArchKey(t *testing.T) {
	key, err := PlatformKey("linux", "amd64")
	if err != nil {
		t.Fatalf("PlatformKey returned error: %v", err)
	}

	if key != "linux-amd64" {
		t.Fatalf("PlatformKey() = %q, want %q", key, "linux-amd64")
	}
}

func TestPlatformKeyRejectsEmptyOS(t *testing.T) {
	if _, err := PlatformKey("", "amd64"); err == nil {
		t.Fatal("PlatformKey returned nil error for empty OS")
	}
}

func TestPlatformKeyRejectsEmptyArch(t *testing.T) {
	if _, err := PlatformKey("linux", ""); err == nil {
		t.Fatal("PlatformKey returned nil error for empty arch")
	}
}

func TestCurrentPlatformKeyReturnsRuntimePlatform(t *testing.T) {
	key, err := CurrentPlatformKey()
	if err != nil {
		t.Fatalf("CurrentPlatformKey returned error: %v", err)
	}

	if key == "" {
		t.Fatal("CurrentPlatformKey returned empty key")
	}

	if !strings.Contains(key, runtime.GOOS+"-"+runtime.GOARCH) {
		t.Fatalf("CurrentPlatformKey() = %q, want to contain %q", key, runtime.GOOS+"-"+runtime.GOARCH)
	}
}
