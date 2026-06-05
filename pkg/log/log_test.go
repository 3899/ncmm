// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"log/slog"
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
	Fatal("hello fatal")
}
