// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package ncm

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Benchmark_Open(b *testing.B) {
	b.ReportAllocs()
	var ncmName = "./testdata/BOE - 822.ncm"
	for i := 0; i < b.N; i++ {
		func() {
			file, err := Open(ncmName)
			defer file.Close()
			assert.NoError(b, err)
			assert.NoError(b, file.DecodeCover(io.Discard))
			assert.NoError(b, file.DecodeMusic(io.Discard))
			assert.NoError(b, file.DecodeCover(io.Discard))
		}()
	}
}
