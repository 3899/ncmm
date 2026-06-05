// Copyright (c) 2026 @3899. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package ncmm

import (
	"github.com/3899/ncmm/pkg/log"

	"github.com/spf13/cobra"
)

type Login struct {
	root *Root
	cmd  *cobra.Command
	l    *log.Logger
}

func NewLogin(root *Root, l *log.Logger) *Login {
	c := &Login{
		root: root,
		l:    l,
		cmd: &cobra.Command{
			Use:     "login",
			Short:   "Login netease cloud music",
			Example: "  ncmm login -h\n  ncmm login qrcode\n  ncmm login phone\n  ncmm login cookiecloud\n  ncmm login cookie",
		},
	}
	c.addFlags()
	c.Add(qrcode(c, l))
	c.Add(phone(c, l))
	c.Add(cookieCloud(c, l))
	c.Add(cookie(c, l))

	return c
}

func (c *Login) addFlags() {}

func (c *Login) Add(command ...*cobra.Command) {
	c.cmd.AddCommand(command...)
}

func (c *Login) Command() *cobra.Command {
	return c.cmd
}
