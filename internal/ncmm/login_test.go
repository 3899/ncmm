package ncmm

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/3899/ncmm/config"
	"github.com/3899/ncmm/internal/loginresult"
	"github.com/spf13/cobra"
)

func TestSaveLoginResultNoConfigWrite(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, "config.yaml")
	original := config.DefaultYAML()
	if err := os.WriteFile(configPath, original, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.New(configPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ReplaceMagicVariables("HOME", home)
	cfg.Network.Cookie.Filepath = filepath.Join(home, "cookie.json")
	root := &Root{Cfg: cfg, CfgPath: configPath, Opts: RootOpts{Home: home}}
	var output bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&output)
	login := &Login{root: root, cmd: cmd, noConfigWrite: true, jsonResult: true}
	tempCookie := filepath.Join(home, "incoming.cookie")
	if err := os.WriteFile(tempCookie, []byte(`{"MUSIC_U":"test"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := login.saveLoginResult(context.Background(), "tester", 123, tempCookie, "fan1.json", ""); err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(current, original) {
		t.Fatal("--no-config-write modified config.yaml")
	}
	result, err := loginresult.Parse(output.String())
	if err != nil {
		t.Fatal(err)
	}
	if result.UID != 123 || result.Main || filepath.Base(result.CookiePath) != "fan1.json" {
		t.Fatalf("unexpected structured result: %+v", result)
	}
	if _, err := os.Stat(result.CookiePath); err != nil {
		t.Fatalf("Cookie output missing: %v", err)
	}
}
