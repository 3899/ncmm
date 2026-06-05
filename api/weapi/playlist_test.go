// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package weapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlaylist(t *testing.T) {
	var req = PlaylistReq{
		Uid:    "1289504343",
		Offset: "",
		Limit:  "30",
	}
	got, err := cli.Playlist(ctx, &req)
	assert.NoError(t, err)
	t.Logf("Playlist: %+v\n", got)
}
