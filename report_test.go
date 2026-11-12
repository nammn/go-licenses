package main

import "testing"

func TestResolveLicenseURLSkips(t *testing.T) {
	called := false
	got, err := resolveLicenseURL(true, func() (string, error) {
		called = true
		return "https://example.com/LICENSE", nil
	})
	if err != nil {
		t.Fatalf("resolveLicenseURL() error = %v", err)
	}
	if got != UNKNOWN {
		t.Errorf("resolveLicenseURL() URL = %q, want %q", got, UNKNOWN)
	}
	if called {
		t.Error("resolver was called with skip=true")
	}
}
