// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"testing"
)

func init() {
	Default = New(nil)
}

func TestPrint(t *testing.T) {
	Debug("hello debug")
	Info("hello info:%s", "chaunsin")
	InfoW(fmt.Sprintf("hello info:%s", "chaunsin"), "sex", slog.StringValue("man"))

	Default.SetLevel(slog.LevelWarn)
	Info("can not print")
}

func TestFatalExitsWithFailure(t *testing.T) {
	if os.Getenv("NCMM_LOG_FATAL_HELPER") == "1" {
		Fatal("hello fatal")
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestFatalExitsWithFailure$")
	cmd.Env = append(os.Environ(), "NCMM_LOG_FATAL_HELPER=1")
	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("Fatal() exit error = %v; want exit code 1", err)
	}
}
