#!/usr/bin/env python3
# -*- coding: utf-8 -*-

"""
cron: 1 1 1 1 1
new Env('NCMM 账号登录')

====================================================================
NCMM 账号登录脚本配置说明
====================================================================
本脚本支持在青龙面板、呆呆面板 (daidai) 及 Docker 环境变量中配置登录。
按以下优先级自动识别并导入：

【方式 1：CookieCloud 自动同步登录】
在面板环境变量中配置：
  - NCMM_COOKIECLOUD_UUID     : CookieCloud 用户 UUID (必须)
  - NCMM_COOKIECLOUD_PASSWORD : CookieCloud 解密密码 (必须)
  - NCMM_COOKIECLOUD_SERVER   : CookieCloud 服务端地址 (可选，默认 http://127.0.0.1:8088)

【方式 2：环境变量 Cookie 导入（推荐，支持主/辅账号分离）】
在面板环境变量中配置：
  - NCMM_MAIN_COOKIE       : 主账号 Cookie 或 MUSIC_U 值 (单个)
  - NCMM_SECONDARY_COOKIE  : 辅助账号 Cookie 或 MUSIC_U 值 (多个用 &、@ 或换行分隔)
  - NCMM_COOKIE            : (通用变量) 若未分主辅，首个自动为主账号，后续为辅助账号。

【方式 3：本地 Cookie 文件导入】
  - NCMM_MAIN_COOKIE_FILE      : 主账号 Cookie 文件路径 (单个)
  - NCMM_SECONDARY_COOKIE_FILE : 辅助账号 Cookie 文件路径 (多个用逗号 , 分号 ; & 或换行分隔)
  - NCMM_COOKIE_FILE           : (通用变量) 若未分主辅，首个文件为主账号，后续为辅助账号。(默认 cookie.txt)
====================================================================
"""

import os
import sys
import platform
import subprocess

# 获取当前脚本所在的真实目录
current_dir = os.path.dirname(os.path.abspath(__file__))

def get_binary_path():
    """判断二进制文件名并检查是否存在，若不存在则自动调用 ncmm-update.py 下载安装"""
    is_windows = 'windows' in platform.system().lower()
    binary_name = "ncmm.exe" if is_windows else "ncmm"
    binary_path = os.path.join(current_dir, binary_name)
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

        if not os.path.exists(binary_path):
            print(f"[ERROR] 自动运行 ncmm-update.py 完成后，仍未找到 {binary_path}。")
            sys.exit(1)
    return binary_path

def split_items(val, extra_separators=""):
    """拆分环境变量中的多项（支持 &、@、逗号、分号、换行符）"""
    if not val:
        return []
    s = val.replace('&', '\n').replace('@', '\n')
    if ',' in extra_separators:
        s = s.replace(',', '\n')
    if ';' in extra_separators:
        s = s.replace(';', '\n')
    return [line.strip() for line in s.splitlines() if line.strip()]

def login_by_cookiecloud(binary_path):
    """方式 1：通过 CookieCloud 自动同步登录"""
    uuid = os.getenv("NCMM_COOKIECLOUD_UUID") or os.getenv("COOKIECLOUD_UUID")
    password = (os.getenv("NCMM_COOKIECLOUD_PASSWORD") or 
                os.getenv("NCMM_COOKIECLOUD_PASS") or 
                os.getenv("COOKIECLOUD_PASSWORD") or 
                os.getenv("COOKIECLOUD_PASS"))
    server = (os.getenv("NCMM_COOKIECLOUD_SERVER") or 
              os.getenv("COOKIECLOUD_SERVER") or 
              "http://127.0.0.1:8088")

    if uuid and password:
        print(f"[LOG] 检测到 CookieCloud 配置 (UUID: {uuid[:6]}***)，正在从 {server} 同步...")
        cmd = [binary_path, "login", "cookiecloud", "-s", server, "-u", uuid, "-p", password]
        print(f"[LOG] 正在执行: {' '.join(cmd)}")
        res = subprocess.run(cmd, cwd=current_dir)
        return res.returncode == 0
    return False

def format_cookie(c_val):
    """确保 Cookie 拥有 MUSIC_U= 键名"""
    return c_val if "=" in c_val else f"MUSIC_U={c_val}"

def login_by_env_cookie(binary_path):
    """方式 2：通过环境变量 Cookie 导入（支持 NCMM_MAIN_COOKIE / NCMM_SECONDARY_COOKIE 显式分离与 NCMM_COOKIE 通用变量）"""
    main_cookie = os.getenv("NCMM_MAIN_COOKIE")
    sec_cookie = os.getenv("NCMM_SECONDARY_COOKIE")
    gen_cookie = os.getenv("NCMM_COOKIE") or os.getenv("MUSIC_U")

    # 情况 A：显式配置了 NCMM_MAIN_COOKIE 或 NCMM_SECONDARY_COOKIE
    if main_cookie or sec_cookie:
        success_count = 0
        if main_cookie:
            c_str = format_cookie(main_cookie.strip())
            print(f"[LOG] 正在导入主账号 (来自 NCMM_MAIN_COOKIE)...")
            cmd = [binary_path, "login", "cookie", "-m", c_str]
            print(f"[LOG] 正在执行: {' '.join(cmd)}")
            res = subprocess.run(cmd, cwd=current_dir)
            if res.returncode == 0:
                success_count += 1

        if sec_cookie:
            sec_list = split_items(sec_cookie)
            print(f"[LOG] 检测到辅助账号 (NCMM_SECONDARY_COOKIE)，共 {len(sec_list)} 个账号，开始导入...")
            for idx, c_val in enumerate(sec_list, start=1):
                c_str = format_cookie(c_val)
                print(f"[LOG] 正在导入辅助账号 #{idx}...")
                cmd = [binary_path, "login", "cookie", c_str]
                print(f"[LOG] 正在执行: {' '.join(cmd)}")
                res = subprocess.run(cmd, cwd=current_dir)
                if res.returncode == 0:
                    success_count += 1
        return success_count > 0

    # 情况 B：仅配置了通用 NCMM_COOKIE / MUSIC_U 变量
    if gen_cookie:
        cookies = split_items(gen_cookie)
        if not cookies:
            return False

        print(f"[LOG] 检测到通用 Cookie 环境变量 (NCMM_COOKIE)，共包含 {len(cookies)} 个账号...")
        success_count = 0
        for idx, c_val in enumerate(cookies, start=1):
            c_str = format_cookie(c_val)
            cmd = [binary_path, "login", "cookie"]
            if idx == 1:
                cmd.append("-m")
            cmd.append(c_str)

            print(f"[LOG] 正在导入账号 #{idx} ({'主账号' if idx == 1 else '辅助账号'})...")
            print(f"[LOG] 正在执行: {' '.join(cmd)}")
            res = subprocess.run(cmd, cwd=current_dir)
            if res.returncode == 0:
                success_count += 1
        return success_count > 0

    return False

def resolve_path(path_str):
    """将参数转换为规范的绝对路径"""
    if not path_str:
        return ""
    path_str = path_str.strip()
    return path_str if os.path.isabs(path_str) else os.path.join(current_dir, path_str)

def login_by_file(binary_path):
    """方式 3：通过指定的本地 Cookie 文件导入 (支持 NCMM_MAIN_COOKIE_FILE / NCMM_SECONDARY_COOKIE_FILE 及 NCMM_COOKIE_FILE)"""
    main_file_var = os.getenv("NCMM_MAIN_COOKIE_FILE")
    sec_files_var = os.getenv("NCMM_SECONDARY_COOKIE_FILE")
    gen_file_var = os.getenv("NCMM_COOKIE_FILE") or os.getenv("COOKIE_FILE")

    # 情况 A：显式配置了 NCMM_MAIN_COOKIE_FILE 或 NCMM_SECONDARY_COOKIE_FILE
    if main_file_var or sec_files_var:
        success_count = 0
        if main_file_var:
            main_path = resolve_path(main_file_var)
            if os.path.exists(main_path):
                base_name = os.path.splitext(os.path.basename(main_path))[0]
                json_file = f"{base_name}.json"
                print(f"[LOG] 正在从文件导入主账号 (来自 NCMM_MAIN_COOKIE_FILE): {main_path}")
                cmd = [binary_path, "login", "cookie", "-m", "-f", main_path, "-o", json_file]
                print(f"[LOG] 正在执行: {' '.join(cmd)}")
                res = subprocess.run(cmd, cwd=current_dir)
                if res.returncode == 0:
                    success_count += 1
            else:
                print(f"[WARNING] 指定的主账号 Cookie 文件不存在: {main_path}")

        if sec_files_var:
            sec_file_list = split_items(sec_files_var, extra_separators=",;")
            print(f"[LOG] 检测到辅助账号文件配置 (NCMM_SECONDARY_COOKIE_FILE)，共 {len(sec_file_list)} 个文件...")
            for idx, f_path_str in enumerate(sec_file_list, start=1):
                sec_path = resolve_path(f_path_str)
                if os.path.exists(sec_path):
                    base_name = os.path.splitext(os.path.basename(sec_path))[0]
                    json_file = f"{base_name}.json"
                    print(f"[LOG] 正在从文件导入辅助账号 #{idx}: {sec_path}")
                    cmd = [binary_path, "login", "cookie", "-f", sec_path, "-o", json_file]
                    print(f"[LOG] 正在执行: {' '.join(cmd)}")
                    res = subprocess.run(cmd, cwd=current_dir)
                    if res.returncode == 0:
                        success_count += 1
                else:
                    print(f"[WARNING] 指定的辅助账号 Cookie 文件不存在: {sec_path}")
        return success_count > 0

    # 情况 B：配置了通用 NCMM_COOKIE_FILE（或 COOKIE_FILE），与 NCMM_COOKIE 保持完全相同的逻辑
    if gen_file_var:
        files = split_items(gen_file_var, extra_separators=",;")
        if not files:
            return False

        print(f"[LOG] 检测到通用 Cookie 文件配置 (NCMM_COOKIE_FILE)，共包含 {len(files)} 个文件...")
        success_count = 0
        for idx, f_path_str in enumerate(files, start=1):
            f_path = resolve_path(f_path_str)
            if os.path.exists(f_path):
                base_name = os.path.splitext(os.path.basename(f_path))[0]
                json_file = f"{base_name}.json"
                cmd = [binary_path, "login", "cookie"]
                if idx == 1:
                    cmd.append("-m")
                cmd.extend(["-f", f_path, "-o", json_file])

                print(f"[LOG] 正在从文件导入账号 #{idx} ({'主账号' if idx == 1 else '辅助账号'}): {f_path}")
                print(f"[LOG] 正在执行: {' '.join(cmd)}")
                res = subprocess.run(cmd, cwd=current_dir)
                if res.returncode == 0:
                    success_count += 1
            else:
                print(f"[WARNING] 指定的 Cookie 文件不存在: {f_path}")
        return success_count > 0

    # 情况 C：未配置任何文件环境变量，默认使用同级目录下的 cookie.txt 作为主账号文件
    default_txt = os.path.join(current_dir, "cookie.txt")
    if os.path.exists(default_txt):
        base_name = "cookie"
        json_file = f"{base_name}.json"
        print(f"[LOG] 未配置环境变量，使用默认主账号文件导入: {default_txt}")
        cmd = [binary_path, "login", "cookie", "-m", "-f", default_txt, "-o", json_file]
        print(f"[LOG] 正在执行: {' '.join(cmd)}")
        res = subprocess.run(cmd, cwd=current_dir)
        return res.returncode == 0

    return False

def main():
    binary_path = get_binary_path()

    # 1. 优先尝试 CookieCloud 自动同步
    if login_by_cookiecloud(binary_path):
        sys.exit(0)

    # 2. 尝试环境变量 Cookie (NCMM_MAIN_COOKIE / NCMM_SECONDARY_COOKIE / NCMM_COOKIE)
    if login_by_env_cookie(binary_path):
        sys.exit(0)

    # 3. 尝试本地 Cookie 文件 (NCMM_MAIN_COOKIE_FILE / NCMM_SECONDARY_COOKIE_FILE / NCMM_COOKIE_FILE / cookie.txt)
    if login_by_file(binary_path):
        sys.exit(0)

    # 4. 均未命中，打印排查指引
    print("\n[ERROR] 未找到任何有效的登录信息！")
    print("==================================================")
    print("【青龙/呆呆面板推荐配置方式（在环境变量中添加任一组即可）】:")
    print("  方式 1 (CookieCloud 自动同步):")
    print("    - NCMM_COOKIECLOUD_UUID        : CookieCloud 用户 UUID")
    print("    - NCMM_COOKIECLOUD_PASSWORD    : CookieCloud 解密密码")
    print("    - NCMM_COOKIECLOUD_SERVER      : 服务器地址 (可选，默认 http://127.0.0.1:8088)")
    print("  方式 2 (环境变量 Cookie 导入，推荐):")
    print("    - NCMM_MAIN_COOKIE             : 主账号 Cookie / MUSIC_U")
    print("    - NCMM_SECONDARY_COOKIE        : 辅助账号 Cookie (多个用 & 分隔)")
    print("    - NCMM_COOKIE                  : 通用变量 (首个为主账号，后续为辅助账号)")
    print("  方式 3 (本地文件导入):")
    print("    - NCMM_MAIN_COOKIE_FILE        : 主账号 Cookie 文件路径")
    print("    - NCMM_SECONDARY_COOKIE_FILE   : 辅助账号 Cookie 文件路径 (多个用逗号 , 分隔)")
    print("    - NCMM_COOKIE_FILE             : 通用文件变量 (首个为主账号，后续为辅助账号，默认 cookie.txt)")
    print("==================================================")
    sys.exit(1)

if __name__ == '__main__':
    main()
