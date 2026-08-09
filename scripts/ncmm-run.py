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

# 检查 ncmm 是否存在，若不存在则自动调用 ncmm-update.py 下载安装
if not os.path.exists(binary_path):
    print(f"[LOG] 未在 {current_dir} 找到 {binary_name}，正在自动调用 ncmm-update.py 下载程序...")
    update_script = os.path.join(current_dir, "ncmm-update.py")
    if os.path.exists(update_script):
        update_cmd = [sys.executable, update_script]
        print(f"[LOG] 正在自动执行安装: {' '.join(update_cmd)}")
        res = subprocess.run(update_cmd, cwd=current_dir)
        if res.returncode != 0:
            print(f"[ERROR] 自动调用 ncmm-update.py 安装失败 (exit code: {res.returncode})，请检查网络或配置。")
            sys.exit(res.returncode)
    else:
        print(f"[ERROR] 未找到安装脚本 {update_script}，无法自动下载。")
        sys.exit(1)

    # 再次检查二进制文件是否存在
    if not os.path.exists(binary_path):
        print(f"[ERROR] 自动运行 ncmm-update.py 完成后，仍未找到 {binary_path}。")
        sys.exit(1)

# 运行 ncmm
cmd = [binary_path] + RUN_ARGS
print(f"[LOG] 正在执行: {' '.join(cmd)}")
result = subprocess.run(cmd, cwd=current_dir)
sys.exit(result.returncode)
