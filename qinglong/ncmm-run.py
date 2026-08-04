#!/usr/bin/env python3
# -*- coding: utf-8 -*-

"""
cron: 9 0,13 * * *
new Env('NCMM 任务执行')
"""

import os
import sys
import platform
import subprocess

# ============ 自定义运行参数 ============
# 修改此处可自定义 ncmm 的运行命令，默认为 ["task"]，即执行 ./ncmm task
# 示例:
#   RUN_ARGS = ["task"]                        => ./ncmm task
#   RUN_ARGS = ["task", "--sign", "--playids"]  => ./ncmm task --sign --playids
RUN_ARGS = ["task"]
# ========================================

# 获取当前脚本所在的真实目录
current_dir = os.path.dirname(os.path.abspath(__file__))

# 判断二进制文件名
is_windows = 'windows' in platform.system().lower()
binary_name = "ncmm.exe" if is_windows else "ncmm"
binary_path = os.path.join(current_dir, binary_name)

# 检查 ncmm 是否存在
if not os.path.exists(binary_path):
    print(f"[ERROR] 未找到 {binary_name}，请先运行 ncmm-update.py 下载程序。")
    print(f"  python3 {os.path.join(current_dir, 'ncmm-update.py')}")
    sys.exit(1)

# 运行 ncmm
cmd = [binary_path] + RUN_ARGS
print(f"[LOG] 正在执行: {' '.join(cmd)}")
result = subprocess.run(cmd, cwd=current_dir)
sys.exit(result.returncode)
