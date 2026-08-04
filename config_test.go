package kag3

import "testing"

func TestLoadDefaultContinueMarkStyleDefaultsToFollow(t *testing.T) {
	c := &Config{}
	c.LoadDefault()
	if c.ContinueMarkStyle != "follow" {
		t.Errorf("ContinueMarkStyle = %q, want %q (backward-compatible default)", c.ContinueMarkStyle, "follow")
	}
}
