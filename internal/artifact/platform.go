package artifact

import (
	"fmt"
	"path/filepath"
	"runtime"
)

var supportedPlatforms = []string{
	"linux-amd64",
	"linux-arm64",
	"darwin-amd64",
	"darwin-arm64",
}

// PlatformKey returns the canonical artifact platform key for a Go OS and arch.
func PlatformKey(goos, goarch string) (string, error) {
	if goos == "" {
		return "", fmt.Errorf("goos is required")
	}
	if goarch == "" {
		return "", fmt.Errorf("goarch is required")
	}

	return goos + "-" + goarch, nil
}

// CurrentPlatformKey returns the artifact platform key for the current runtime.
func CurrentPlatformKey() (string, error) {
	return PlatformKey(runtime.GOOS, runtime.GOARCH)
}

// SupportedPlatforms returns the stable ordered list of platforms supported by release artifacts.
func SupportedPlatforms() []string {
	platforms := make([]string, len(supportedPlatforms))
	copy(platforms, supportedPlatforms)
	return platforms
}

// IsSupportedPlatform reports whether platform is supported by release artifacts.
func IsSupportedPlatform(platform string) bool {
	for _, supportedPlatform := range supportedPlatforms {
		if platform == supportedPlatform {
			return true
		}
	}

	return false
}

// ArtifactName returns the release artifact name for version and platform.
func ArtifactName(version, platform string) (string, error) {
	if version == "" {
		return "", fmt.Errorf("version is required")
	}
	if !IsSupportedPlatform(platform) {
		return "", fmt.Errorf("unsupported platform %q", platform)
	}

	return "opentunnel-" + version + "-" + platform, nil
}

// ArtifactPath returns the release artifact path under dir for version and platform.
func ArtifactPath(dir, version, platform string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("dir is required")
	}

	name, err := ArtifactName(version, platform)
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, name), nil
}
