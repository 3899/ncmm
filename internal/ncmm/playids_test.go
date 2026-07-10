// Copyright (c) 2026 xxx. All rights reserved.
// Use of this source code is governed by a MIT-style license that can be found in the LICENSE file.

package ncmm

import (
	"errors"
	"testing"
)

// TestIsDatabaseLockErrorMatched 验证数据库目录锁占用时会进入无缓存播放分支
// 参数：t 表示测试上下文
// 返回：无
func TestIsDatabaseLockErrorMatched(t *testing.T) {
	cases := []error{
		errors.New(`failed to open badger db: Cannot acquire directory lock on "./database/badger/". Another process is using this Badger database. err: resource temporarily unavailable`),
		errors.New("open LOCK: process cannot access the file because it is being used by another process"),
	}

	for _, err := range cases {
		// 1. 数据库目录锁被其他运行实例占用时，播放任务可以降级为无缓存模式
		if !isDatabaseLockError(err) {
			t.Fatalf("expected lock error to be matched: %v", err)
		}
	}
}

// TestIsDatabaseLockErrorIgnoresConfigErrors 验证非锁类数据库异常仍然暴露给调用方
// 参数：t 表示测试上下文
// 返回：无
func TestIsDatabaseLockErrorIgnoresConfigErrors(t *testing.T) {
	cases := []error{
		nil,
		errors.New("unsupported database driver: sqlite"),
		errors.New("permission denied"),
	}

	for _, err := range cases {
		// 1. 配置错误和权限错误不是可安全降级的缓存锁冲突
		if isDatabaseLockError(err) {
			t.Fatalf("expected non-lock error to be ignored: %v", err)
		}
	}
}
