//go:build windows

package ncmm

import (
	"reflect"
	"testing"
)

func TestWithoutBackgroundFlag(t *testing.T) {
	args := []string{"--home", `C:\ncmm`, "web", "--background", "--listen", "127.0.0.1:3899"}
	want := []string{"--home", `C:\ncmm`, "web", "--listen", "127.0.0.1:3899"}
	if got := withoutBackgroundFlag(args); !reflect.DeepEqual(got, want) {
		t.Fatalf("withoutBackgroundFlag() = %v; want %v", got, want)
	}
}

func TestBackgroundReadyURL(t *testing.T) {
	tests := map[string]string{
		"127.0.0.1:3899": "http://127.0.0.1:3899/api/v1/instance",
		"0.0.0.0:3899":   "http://127.0.0.1:3899/api/v1/instance",
		"[::]:3899":      "http://127.0.0.1:3899/api/v1/instance",
	}
	for listen, want := range tests {
		got, err := backgroundReadyURL(listen)
		if err != nil || got != want {
			t.Fatalf("backgroundReadyURL(%q) = %q, %v; want %q", listen, got, err, want)
		}
	}
}
