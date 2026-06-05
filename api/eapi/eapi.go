// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package eapi

import "github.com/3899/ncmm/api"

type Api struct {
	client *api.Client
}

func New(client *api.Client) *Api {
	a := Api{client: client}
	return &a
}
