// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package ncmm

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestParsePluginName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ncmm-sample.exe", "sample"},
		{"ncmm-sample", "sample"},
		{"ncmm_sample.exe", "sample"},
		{"sample.exe", "sample"},
		{"sample", "sample"},
		{"ncmm-daily-share.exe", "daily-share"},
	}

	for _, tc := range tests {
		got := parsePluginName(tc.input)
		if got != tc.want {
			t.Errorf("parsePluginName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestIsExecutableFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		if !isExecutableFile("foo.exe") {
			t.Errorf("foo.exe should be executable on windows")
		}
		if !isExecutableFile("foo.bat") {
			t.Errorf("foo.bat should be executable on windows")
		}
		if isExecutableFile("foo.go") {
			t.Errorf("foo.go should not be executable")
		}
	} else {
		if isExecutableFile("foo.go") {
			t.Errorf("foo.go should not be executable")
		}
		if isExecutableFile("foo.md") {
			t.Errorf("foo.md should not be executable")
		}
	}
}

func TestPluginDiscoveryAndExecution(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ncmm_plugin_test")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	pluginsDir := filepath.Join(tmpDir, "plugins")
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	var scriptName, scriptContent string
	if runtime.GOOS == "windows" {
		scriptName = "ncmm-mock.bat"
		scriptContent = "@echo off\r\necho Hello from mock plugin\r\necho {\"success\":true,\"message\":\"mock success\"}\r\n"
	} else {
		scriptName = "ncmm-mock.sh"
		scriptContent = "#!/bin/sh\necho 'Hello from mock plugin'\necho '{\"success\":true,\"message\":\"mock success\"}'\n"
	}

	scriptPath := filepath.Join(pluginsDir, scriptName)
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	root := &Root{
		Opts: RootOpts{Home: tmpDir},
	}
	p := NewPlugin(root, nil)

	plugins := p.DiscoverPlugins()
	found := false
	for _, plug := range plugins {
		if plug.Name == "mock" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected to discover 'mock' plugin, got: %+v", plugins)
	}

	ctx := context.Background()
	res, err := p.ExecutePlugin(ctx, scriptPath, &PluginContext{
		Home: tmpDir,
	})
	if err != nil {
		t.Fatalf("ExecutePlugin failed: %v", err)
	}
	if !res.Success || res.Message != "mock success" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestPluginRunCommandExtraArgs(t *testing.T) {
	root := &Root{}
	p := NewPlugin(root, nil)
	runCmd := p.runCommand()

	// Should show help if called with -h / --help without error
	runCmd.SetArgs([]string{"--help"})
	if err := runCmd.Execute(); err != nil {
		t.Fatalf("run --help failed: %v", err)
	}
}

