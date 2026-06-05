// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package weapi

import (
	"context"
	"os"
	"testing"

	"github.com/3899/ncmm/api"
	"github.com/3899/ncmm/pkg/cookie"
	"github.com/3899/ncmm/pkg/log"
)

var (
	cli *Api
	ctx = context.TODO()
)

func TestMain(t *testing.M) {
	log.Default = log.New(&log.Config{
		Level:  "debug",
		Stdout: true,
	})

	cfg := api.Config{
		Debug:   true,
		Timeout: 0,
		Retry:   0,
		Cookie: cookie.Config{
			Options:  nil,
			Filepath: "../../testdata/cookie.json",
			Interval: 0,
		},
	}
	client := api.New(&cfg)
	cli = New(client)
	os.Exit(t.Run())
}
