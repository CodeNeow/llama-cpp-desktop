package core

import "testing"

// TestEffectiveHost verifies effectiveHost pure function: lan → 0.0.0.0,
// all other values (including empty and invalid) → 127.0.0.1.
// Shared by SaveServerConfig / loadConfig / buildServerCommand for consistent host derivation.
func TestEffectiveHost(t *testing.T) {
	cases := []struct {
		mode string
		want string
	}{
		{accessLocal, "127.0.0.1"},
		{accessLAN, "0.0.0.0"},
		{"", "127.0.0.1"},       // empty string falls back to loopback
		{"local ", "127.0.0.1"}, // whitespace-padded invalid value falls back to loopback
		{"wan", "127.0.0.1"},    // out-of-whitelist value falls back to loopback
		{"0.0.0.0", "127.0.0.1"},
	}
	for _, c := range cases {
		if got := effectiveHost(c.mode); got != c.want {
			t.Errorf("effectiveHost(%q) = %q, want %q", c.mode, got, c.want)
		}
	}
}
