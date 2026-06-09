package artifact

import (
	"fmt"
	"runtime"
)

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
