#!/usr/bin/env python3
# -*- coding: utf-8 -*-

"""
cron: 1 1 1 1 1
new Env('NCMM 账号登录导入')
"""

import os
import sys
import platform
import subprocess

# ============ 自定义登录配置 ============
# 输入的 Cookie TXT 文件名（位于脚本同级目录或指定相对路径）
# 导出的 JSON 文件名会自动与 TXT 主文件名保持一致（例如 "cookie.txt" => "cookie.json"）
COOKIE_TXT_FILE = "cookie.txt"
# ========================================

# 获取当前脚本所在的真实目录
current_dir = os.path.dirname(os.path.abspath(__file__))

# 确定 txt 完整路径及自动推导的 json 文件名
txt_path = os.path.join(current_dir, COOKIE_TXT_FILE)
base_name = os.path.splitext(COOKIE_TXT_FILE)[0]
json_file = f"{base_name}.json"

# 1. 检查 Cookie TXT 文件是否存在
if not os.path.exists(txt_path):
    print(f"[ERROR] 未找到 Cookie TXT 文件: {COOKIE_TXT_FILE}")
    print(f"  请先在 {current_dir} 目录下创建并填入 Cookie 内容。")
    sys.exit(1)

# 2. 判断二进制文件名
is_windows = 'windows' in platform.system().lower()
binary_name = "ncmm.exe" if is_windows else "ncmm"
binary_path = os.path.join(current_dir, binary_name)

# 3. 检查 ncmm 二进制是否存在
if not os.path.exists(binary_path):
    print(f"[ERROR] 未找到 {binary_name}，请先运行 ncmm-update.py 下载程序。")
    print(f"  python3 {os.path.join(current_dir, 'ncmm-update.py')}")
    sys.exit(1)

# 4. 运行 ncmm login cookie -f xxx.txt -o xxx.json
cmd = [binary_path, "login", "cookie", "-f", COOKIE_TXT_FILE, "-o", json_file]
print(f"[LOG] 正在执行: {' '.join(cmd)}")
result = subprocess.run(cmd, cwd=current_dir)
sys.exit(result.returncode)
