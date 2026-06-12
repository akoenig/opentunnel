package originurl

import "testing"

func TestValidateAcceptsHTTPOrigins(t *testing.T) {
	tests := []string{
		"http://localhost:8080",
		"https://relay.example.com",
		"https://[::1]:8443",
	}

	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if err := Validate(raw, "relay origin"); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestValidateRejectsUnsafeOrigins(t *testing.T) {
	tests := []string{
		"http://example.test/$(id)",
		"http://example.test/path",
		"http://example.test?download=true",
		"http://example.test#fragment",
		"http://user@example.test",
		"ftp://example.test",
		"http:///missing-host",
		"-http://example.test",
	}

	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if err := Validate(raw, "relay origin"); err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
		})
	}
}
