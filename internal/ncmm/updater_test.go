// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package ncmm

import (
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int // 1: v1 > v2, -1: v1 < v2, 0: v1 == v2
	}{
		{"v1.1.7", "v1.1.6", 1},
		{"1.1.7", "v1.1.7", 0},
		{"v1.1.6", "v1.1.7", -1},
		{"v1.1.10", "v1.1.9", 1},
		{"1.2.0", "v1.1.7", 1},
		{"1.0.0", "2.0.0", -1},
		{"v1.1.7-beta", "v1.1.7", 0}, // β is stripped/ignored, comparing digits
		{"v1", "v2", -1},
		{"v1.1", "v1.1.0", 0},
		{"v1.1.0", "v1.1", 0},
	}

	for _, tc := range tests {
		res := CompareVersions(tc.v1, tc.v2)
		if res != tc.expected {
			t.Errorf("CompareVersions(%q, %q) = %d, expected %d", tc.v1, tc.v2, res, tc.expected)
		}
	}
}

func TestApplyAvailableUpdateRejectsOfficialDocker(t *testing.T) {
	t.Setenv("NCMM_DOCKER_OFFICIAL", "1")
	root := &Root{AppVersion: "1.1.14", Opts: RootOpts{Home: t.TempDir()}}
	_, err := root.ApplyAvailableUpdate()
	if err == nil || !strings.Contains(err.Error(), "Docker") {
		t.Fatalf("ApplyAvailableUpdate() error = %v; want Docker rejection", err)
	}
}
