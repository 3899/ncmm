// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package main

import (
	_ "embed"
	"strings"

	"github.com/3899/ncmm/internal/ncmm"
)

//go:embed VERSION
var versionStr string

var (
	Version   string
	Commit    = "none"
	BuildTime = "now"
)

func init() {
	Version = strings.TrimSpace(versionStr)
}

func main() {
	c := ncmm.New()
	c.Version(Version, BuildTime, Commit)
	c.Execute()
}
