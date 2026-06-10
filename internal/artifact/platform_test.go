package artifact

import (
	"path/filepath"
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

func TestSupportedPlatformsContainsV1UnixAndDarwinPlatforms(t *testing.T) {
	want := []string{"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64"}
	got := SupportedPlatforms()
	if len(got) != len(want) {
		t.Fatalf("SupportedPlatforms length = %d, want %d: %v", len(got), len(want), got)
	}
	for index, platform := range want {
		if got[index] != platform {
			t.Fatalf("SupportedPlatforms()[%d] = %q, want %q", index, got[index], platform)
		}
	}
}

func TestIsSupportedPlatform(t *testing.T) {
	if !IsSupportedPlatform("linux-amd64") {
		t.Fatal("linux-amd64 should be supported")
	}
	if IsSupportedPlatform("windows-amd64") {
		t.Fatal("windows-amd64 should not be supported in v1")
	}
}

func TestArtifactNameIncludesVersionAndPlatform(t *testing.T) {
	name, err := ArtifactName("1.0.0", "darwin-arm64")
	if err != nil {
		t.Fatalf("ArtifactName returned error: %v", err)
	}
	if name != "opentunnel-1.0.0-darwin-arm64" {
		t.Fatalf("ArtifactName = %q", name)
	}
}

func TestArtifactNameRejectsUnsupportedPlatform(t *testing.T) {
	if _, err := ArtifactName("1.0.0", "windows-amd64"); err == nil {
		t.Fatal("ArtifactName returned nil error for unsupported platform")
	}
}

func TestArtifactNameRejectsEmptyVersion(t *testing.T) {
	if _, err := ArtifactName("", "linux-amd64"); err == nil {
		t.Fatal("ArtifactName returned nil error for empty version")
	}
}

func TestArtifactPathJoinsDirWithArtifactName(t *testing.T) {
	path, err := ArtifactPath("dist", "1.0.0", "linux-amd64")
	if err != nil {
		t.Fatalf("ArtifactPath returned error: %v", err)
	}

	want := filepath.Join("dist", "opentunnel-1.0.0-linux-amd64")
	if path != want {
		t.Fatalf("ArtifactPath = %q, want %q", path, want)
	}
}

func TestArtifactPathRejectsEmptyDir(t *testing.T) {
	if _, err := ArtifactPath("", "1.0.0", "linux-amd64"); err == nil {
		t.Fatal("ArtifactPath returned nil error for empty dir")
	}
}
