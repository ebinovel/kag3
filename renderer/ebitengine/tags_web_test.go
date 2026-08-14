package ebitengine

import (
	"testing"

	"github.com/ebinovel/kag3"
)

// TestHandleWebOpensURL covers [web url=]'s happy path: it goes through the
// shared openURL (openurl.go), which this test observes via launchURL.
func TestHandleWebOpensURL(t *testing.T) {
	origLaunch := launchURL
	defer func() { launchURL = origLaunch }()

	var got string
	launchURL = func(rawURL string) error {
		got = rawURL
		return nil
	}

	r := newTestRenderer()
	tag := kag3.TagObject{Name: "web", Pm: map[string]string{"url": "https://tyrano.jp/home/example"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("web dispatch error: %v", err)
	}
	if got != "https://tyrano.jp/home/example" {
		t.Errorf("launchURL called with %q, want the tag's url=", got)
	}
}

// TestHandleWebMissingURLErrors covers [web] with no url= at all — a
// script-authoring mistake with nothing to open, matching [movie]'s own
// storage= requirement (a hard error, not a silent no-op).
func TestHandleWebMissingURLErrors(t *testing.T) {
	r := newTestRenderer()
	tag := kag3.TagObject{Name: "web", Pm: map[string]string{}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err == nil {
		t.Fatal("web dispatch with no url= returned nil, want an error")
	}
}

// TestHandleWebRejectedSchemeDoesNotError covers a rejected scheme (openURL
// itself returning an error): [web] must log and continue, not crash the
// coroutine over something that never affects story state.
func TestHandleWebRejectedSchemeDoesNotError(t *testing.T) {
	origLaunch := launchURL
	defer func() { launchURL = origLaunch }()
	launchURL = func(rawURL string) error {
		t.Error("launchURL must not be called for a rejected scheme")
		return nil
	}

	r := newTestRenderer()
	tag := kag3.TagObject{Name: "web", Pm: map[string]string{"url": "file:///etc/passwd"}}
	i := 0
	if err := dispatchTag(r, fakeYield(), tag, &i, 0); err != nil {
		t.Fatalf("web dispatch error = %v, want nil (rejected scheme should be logged and skipped)", err)
	}
}
