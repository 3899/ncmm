package webui

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestPlayStatsRuntimeConfig(t *testing.T) {
	home := t.TempDir()
	document := configDocument{Data: map[string]any{
		"accounts": map[string]any{
			"main":      "./cookie.json",
			"secondary": []any{"./fan1.json", "", 42},
		},
		"database": map[string]any{"driver": "badger", "path": "./database/badger"},
	}}
	config, accounts, err := playStatsRuntimeConfig(document, home)
	if err != nil {
		t.Fatal(err)
	}
	if config.Driver != "badger" || config.Path != filepath.Join(home, "database", "badger") {
		t.Fatalf("unexpected database config: %+v", config)
	}
	wantAccounts := []string{"./cookie.json", "./fan1.json"}
	if !reflect.DeepEqual(accounts, wantAccounts) {
		t.Fatalf("accounts = %v, want %v", accounts, wantAccounts)
	}
}
