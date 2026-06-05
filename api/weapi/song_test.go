// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package weapi

import (
	"testing"

	"github.com/3899/ncmm/api/types"

	"github.com/stretchr/testify/assert"
)

func TestSongPlayer(t *testing.T) {
	got, err := cli.SongPlayer(ctx, &SongPlayerReq{Ids: types.IntsString{2115747785}, Br: "128000"})
	assert.NoError(t, err)
	t.Logf("resp:%+v\n", got)
}
