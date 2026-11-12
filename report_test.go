package main

import (
	"context"
	"errors"
	"testing"
)

func TestResolveLicenseURL(t *testing.T) {
	resolveErr := errors.New("resolver failed")
	tests := []struct {
		name        string
		skip        bool
		licenseFile string
		resolveErr  error
		wantURL     string
		wantErr     error
		wantCalled  bool
	}{
		{
			name:        "skip avoids resolving the URL",
			skip:        true,
			licenseFile: "LICENSE",
			wantURL:     UNKNOWN,
		},
		{
			name:        "no skip resolves the URL",
			skip:        false,
			licenseFile: "LICENSE",
			wantURL:     "https://example.com/LICENSE",
			wantCalled:  true,
		},
		{
			name:        "empty license file avoids resolving the URL",
			skip:        false,
			licenseFile: "",
			wantURL:     UNKNOWN,
		},
		{
			name:        "resolver error is returned",
			skip:        false,
			licenseFile: "LICENSE",
			resolveErr:  resolveErr,
			wantURL:     UNKNOWN,
			wantErr:     resolveErr,
			wantCalled:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			resolve := func(context.Context, string) (string, error) {
				called = true
				return "https://example.com/LICENSE", tt.resolveErr
			}

			gotURL, gotErr := resolveLicenseURL(context.Background(), tt.licenseFile, tt.skip, resolve)
			if !errors.Is(gotErr, tt.wantErr) {
				t.Fatalf("resolveLicenseURL() error = %v, want %v", gotErr, tt.wantErr)
			}
			if gotURL != tt.wantURL {
				t.Errorf("resolveLicenseURL() URL = %q, want %q", gotURL, tt.wantURL)
			}
			if called != tt.wantCalled {
				t.Errorf("resolver called = %v, want %v", called, tt.wantCalled)
			}
		})
	}
}
