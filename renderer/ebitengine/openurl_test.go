package ebitengine

import (
	"errors"
	"testing"
)

// TestOpenURLAllowsHTTPAndHTTPS covers openURL's scheme allowlist: http/https
// pass through to launchURL unmodified.
func TestOpenURLAllowsHTTPAndHTTPS(t *testing.T) {
	orig := launchURL
	defer func() { launchURL = orig }()

	for _, want := range []string{"http://tyrano.jp/home/example", "https://tyrano.jp/home/example"} {
		var got string
		launchURL = func(rawURL string) error {
			got = rawURL
			return nil
		}
		if err := openURL(want); err != nil {
			t.Fatalf("openURL(%q) = %v, want nil", want, err)
		}
		if got != want {
			t.Errorf("launchURL called with %q, want %q", got, want)
		}
	}
}

// TestOpenURLRejectsOtherSchemes is the security check behind openURL:
// a .ks-supplied URL must never reach the OS's own scheme dispatch
// (rundll32/open/xdg-open) with anything other than http/https, since that
// dispatch itself can act on file:// paths or OS-specific pseudo-schemes.
func TestOpenURLRejectsOtherSchemes(t *testing.T) {
	orig := launchURL
	defer func() { launchURL = orig }()

	called := false
	launchURL = func(rawURL string) error {
		called = true
		return nil
	}

	for _, bad := range []string{"file:///etc/passwd", "javascript:alert(1)", "no-scheme-at-all", ""} {
		if err := openURL(bad); err == nil {
			t.Errorf("openURL(%q) = nil, want a rejection error", bad)
		}
	}
	if called {
		t.Error("launchURL was called despite every input having a rejected/missing scheme")
	}
}

// TestOpenURLPropagatesLaunchError covers the case where launchURL itself
// fails (e.g. the OS command couldn't start) — openURL must surface that,
// not swallow it silently.
func TestOpenURLPropagatesLaunchError(t *testing.T) {
	orig := launchURL
	defer func() { launchURL = orig }()

	wantErr := errors.New("boom")
	launchURL = func(rawURL string) error { return wantErr }

	if err := openURL("https://tyrano.jp/home/example"); !errors.Is(err, wantErr) {
		t.Errorf("openURL error = %v, want %v", err, wantErr)
	}
}
