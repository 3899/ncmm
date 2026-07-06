package api

import "testing"

func TestUserAgentFallbacks(t *testing.T) {
	client := &Client{cfg: &Config{}}
	if got := client.UserAgent(CryptoModeWEAPI); got != DefaultWEAPIUserAgent {
		t.Fatalf("WEAPI default fallback mismatch: %q", got)
	}
	if got := client.UserAgent(CryptoModeEAPI); got != DefaultEAPIUserAgent {
		t.Fatalf("EAPI default fallback mismatch: %q", got)
	}
	if got := client.UserAgent(CryptoModeXEAPI); got != "" {
		t.Fatalf("XEAPI should not fallback when empty, got %q", got)
	}

	client.cfg.UserAgent.Default = "default-UA"
	if got := client.UserAgent(CryptoModeWEAPI); got != "default-UA" {
		t.Fatalf("WEAPI configured default fallback mismatch: %q", got)
	}
	if got := client.UserAgent(CryptoModeEAPI); got != "default-UA" {
		t.Fatalf("EAPI configured default fallback mismatch: %q", got)
	}
	if got := client.UserAgent(CryptoModeXEAPI); got != "" {
		t.Fatalf("XEAPI should not use configured default fallback, got %q", got)
	}

	client.cfg.UserAgent.XEAPI = "xeapi-UA"
	if got := client.UserAgent(CryptoModeXEAPI); got != "xeapi-UA" {
		t.Fatalf("XEAPI explicit UA mismatch: %q", got)
	}
}
