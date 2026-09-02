package loginresult

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteAndParse(t *testing.T) {
	want := Result{UID: 123, Nickname: "tester", AvatarURL: "https://p1.music.126.net/avatar.jpg", CookiePath: "C:\\data\\fan.json", AccountPath: "${HOME}/fan.json", Main: false}
	var output bytes.Buffer
	output.WriteString("ordinary command output\n")
	if err := Write(&output, want); err != nil {
		t.Fatal(err)
	}
	got, err := Parse(output.String())
	if err != nil {
		t.Fatal(err)
	}
	if got.UID != want.UID || got.Nickname != want.Nickname || got.AvatarURL != want.AvatarURL || got.CookiePath != want.CookiePath || got.AccountPath != want.AccountPath || got.Main != want.Main {
		t.Fatalf("Parse() = %+v; want %+v", got, want)
	}
}

func TestParseRejectsMissingAndIncompleteResults(t *testing.T) {
	if _, err := Parse("login success\n"); err == nil {
		t.Fatal("Parse() accepted missing structured result")
	}
	if _, err := Parse(Marker + "{\"version\":1}"); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("Parse() error = %v; want incomplete result", err)
	}
}
