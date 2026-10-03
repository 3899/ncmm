#!/usr/bin/env python3
# -*- coding: utf-8 -*-

"""
cron: 1 1 1 1 1
new Env('NCMM 安装、更新')

更新规则（重点）：
  本脚本**不维护任何自己的更新规则** —— 版本判断、下载源、加速镜像、解压、二进制替换、
  配置结构升级全部由 ncmm 自己决定，与 ncmm 的自动更新是同一套规则。

  * 已经装好 ncmm：进入 ncmm 程序目录执行 `ncmm update --apply`，
    脚本只负责"手动触发 + 日志回显 + 记录 VERSION"。
  * 还没有 ncmm：没有任何可调用的程序，只能由脚本下载 release 安装包完成**首次安装**
    （仅此一处保留脚本自身的下载逻辑，且只在没有二进制时才会走到）。
"""

import os
import sys
import platform
import urllib.request
import json
import re
import shutil
import zipfile
import tarfile
import time
import subprocess

# 1. 稳定获取当前脚本所在的真实目录
current_dir = os.path.dirname(os.path.abspath(__file__))
target_dir = current_dir  # 直接使用脚本所在目录作为目标目录

print(f"[LOG] Python脚本所在目录: {current_dir}")
print(f"[LOG] 目标二进制目录: {target_dir}")

# 手动触发更新时给 ncmm 的超时（秒）
NCMM_UPDATE_TIMEOUT = 600

# 2. 识别系统平台与架构判定逻辑（仅首次安装时使用）
def get_platform_info():
    sys_name = platform.system().lower()
    arch_name = platform.machine().lower()
    
    if 'windows' in sys_name:
        os_part = "Windows"
        ext = ".zip"
    elif 'linux' in sys_name:
        os_part = "Linux"
        ext = ".tar.gz"
    elif 'darwin' in sys_name:
        os_part = "Darwin"
        ext = ".tar.gz"
    else:
        os_part = sys_name.capitalize()
        ext = ".tar.gz"
        
    if arch_name in ['x86_64', 'amd64']:
        arch_part = "x86_64"
    elif arch_name in ['arm64', 'aarch64']:
        arch_part = "arm64"
    elif 'arm' in arch_name:
        arch_part = "armv6"
    else:
        arch_part = arch_name
        
    return os_part, arch_part, ext

# 3. 获取 GitHub 最新 Release 标签与资产列表（仅首次安装时使用）
def get_latest_release():
    headers = {
        'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36'
    }
    
    # 1. 尝试 GitHub API
    api_urls = [
        "https://api.github.com/repos/3899/ncmm/releases/latest",
        "https://gh-proxy.com/https://api.github.com/repos/3899/ncmm/releases/latest"
    ]
    for url in api_urls:
        try:
            print(f"[LOG] 正在请求 GitHub API 获取最新版本 ({url})...")
            req = urllib.request.Request(url, headers=headers)
            with urllib.request.urlopen(req, timeout=10) as response:
                data = json.loads(response.read().decode('utf-8'))
                tag_name = data.get('tag_name')
                assets = data.get('assets', [])
                if tag_name:
                    return tag_name, assets
        except Exception as e:
            print(f"[WARNING] 请求 GitHub API 失败 ({url}): {e}")
            
    # 2. 备用方案：重定向解析 (依次尝试加速镜像与 GitHub 原地址兜底)
    redirect_urls = [
        "https://gh-proxy.com/https://github.com/3899/ncmm/releases/latest",
        "https://ghproxy.net/https://github.com/3899/ncmm/releases/latest",
        "https://githubproxy.cc/https://github.com/3899/ncmm/releases/latest",
        "https://github.com/3899/ncmm/releases/latest"
    ]
    for url in redirect_urls:
        try:
            print(f"[LOG] 正在通过网页重定向获取最新版本 ({url})...")
            req = urllib.request.Request(url, headers=headers)
            with urllib.request.urlopen(req, timeout=10) as response:
                final_url = response.geturl()
                match = re.search(r'/releases/tag/([^/]+)', final_url)
                if match:
                    tag_name = match.group(1)
                    return tag_name, None
        except Exception as e:
            print(f"[WARNING] 重定向方案获取失败 ({url}): {e}")
        
    return None, None

# 4. 进程查杀释放逻辑（升级失败时的兜底，用于释放被占用的二进制文件）
def stop_running_ncmm(binary_name):
    sys_name = platform.system().lower()
    if 'windows' in sys_name:
        try:
            print("[LOG] 正在检查并强制终止 Windows 平台上的 ncmm.exe 进程...")
            # Windows 平台下，使用 taskkill 强行杀掉对应名称的进程
            subprocess.run(["taskkill", "/F", "/IM", binary_name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        except Exception as e:
            print(f"[WARNING] 尝试终止 Windows 进程时发生异常: {e}")
    else:
        print(f"[LOG] 正在检查并终止 Linux/Unix 平台上的 {binary_name} 进程...")
        pkill_used = False
        try:
            res = subprocess.run(["pkill", "-x", binary_name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            if res.returncode == 0:
                pkill_used = True
        except Exception:
            pkill_used = False

        if not pkill_used:
            # 纯 Python 扫描 /proc 目录（支持无 ps/pkill/lsof 的极简面板环境，如呆呆面板）
            current_pid = os.getpid()
            parent_pid = os.getppid() if hasattr(os, 'getppid') else -1
            
            try:
                for entry in os.listdir('/proc'):
                    if not entry.isdigit():
                        continue
                    pid = int(entry)
                    
                    # 1. 排除当前 Python 脚本进程及父进程（防止自杀）
                    if pid in (current_pid, parent_pid):
                        continue
                    
                    # 2. 读取该 PID 对应的可执行二进制文件软链接
                    exe_link = f"/proc/{entry}/exe"
                    is_target = False
                    
                    if os.path.exists(exe_link):
                        try:
                            real_exe = os.readlink(exe_link)
                            if os.path.basename(real_exe) == binary_name:
                                is_target = True
                        except Exception:
                            pass
                    
                    # 3. 兜底检查命令行：排除带 .py 的脚本进程
                    if not is_target:
                        try:
                            with open(f"/proc/{entry}/cmdline", "rb") as f:
                                cmd_bytes = f.read()
                            cmd_str = cmd_bytes.decode("utf-8", errors="ignore").replace("\0", " ")
                            if ".py" not in cmd_str:
                                parts = cmd_str.split()
                                if parts and os.path.basename(parts[0]) == binary_name:
                                    is_target = True
                        except Exception:
                            pass

                    # 4. 发送 SIGKILL 终止目标进程
                    if is_target:
                        try:
                            os.kill(pid, 9)
                            print(f"[LOG] 已通过 /proc 扫描成功终止进程 PID={pid}")
                        except Exception as ke:
                            print(f"[WARNING] 终止进程 PID={pid} 失败: {ke}")
            except Exception as e:
                print(f"[WARNING] /proc 扫描终止进程发生异常: {e}")

    # 等待 1.5 秒，确保操作系统完全释放文件锁定
    time.sleep(1.5)

# 5. 带有多镜像重试与原地址兜底的下载模块（仅首次安装时使用）
PROXIES = [
    "https://gh-proxy.com/",
    "https://ghproxy.net/",
    "https://githubproxy.cc/",
    ""  # 原地址直连兜底
]

def download_file_with_fallback(src_url, dst_path, headers=None, timeout=45):
    """
    依次尝试加速镜像与 GitHub 原始地址兜底下载文件
    """
    if not headers:
        headers = {
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36'
        }

    clean_url = src_url
    for p in PROXIES:
        if p and clean_url.startswith(p):
            clean_url = clean_url[len(p):]
            break

    candidate_urls = []
    is_github = any(clean_url.startswith(prefix) for prefix in [
        "https://github.com/",
        "https://raw.githubusercontent.com/",
        "https://objects.githubusercontent.com/"
    ])

    if is_github:
        for prefix in PROXIES:
            candidate_urls.append(f"{prefix}{clean_url}")
    else:
        candidate_urls.append(clean_url)

    for idx, url in enumerate(candidate_urls, start=1):
        is_direct = url == clean_url
        desc = "GitHub 原地址直连" if is_direct else "加速镜像"
        print(f"[LOG] 下载尝试 [{idx}/{len(candidate_urls)}] ({desc}): {url}")
        try:
            req = urllib.request.Request(url, headers=headers)
            with urllib.request.urlopen(req, timeout=timeout) as response, open(dst_path, 'wb') as out_file:
                shutil.copyfileobj(response, out_file)
            print(f"[LOG] 成功下载文件: {url}")
            return True
        except Exception as e:
            print(f"[WARNING] 从 {url} 下载失败: {e}，尝试下一个备用源...")

    return False

# 6. 本地 VERSION 文件读写（仅作记录，供面板展示；是否更新完全由 ncmm 判断）
def read_local_version(version_file):
    if not os.path.exists(version_file):
        return "unknown"
    try:
        with open(version_file, 'r', encoding='utf-8') as f:
            return f.read().strip() or "unknown"
    except Exception as e:
        print(f"[WARNING] 无法读取本地 VERSION 文件: {e}")
        return "unknown"

def write_local_version(version_file, version):
    try:
        with open(version_file, 'w', encoding='utf-8') as f:
            f.write(version.strip().lstrip('vV') + '\n')
        print(f"[LOG] 已更新本地 VERSION 文件为: {version.strip().lstrip('vV')}")
        return True
    except Exception as e:
        print(f"[WARNING] 写入 VERSION 文件失败: {e}")
        return False

# 7. 调用 ncmm 自身完成更新（手动触发的核心）
def run_ncmm_cli(binary_path, args, timeout=NCMM_UPDATE_TIMEOUT, capture=False):
    """在 ncmm 程序目录内执行 ncmm 子命令，返回 (returncode, stdout+stderr)。"""
    print(f"[LOG] 进入程序目录执行: {os.path.basename(binary_path)} {' '.join(args)}")
    try:
        proc = subprocess.run(
            [binary_path] + args,
            cwd=target_dir,
            timeout=timeout,
            stdout=subprocess.PIPE if capture else None,
            stderr=subprocess.STDOUT if capture else None,
            text=True,
        )
        output = (proc.stdout or "") if capture else ""
        return proc.returncode, output
    except subprocess.TimeoutExpired:
        print(f"[ERROR] 执行 ncmm {' '.join(args)} 超时（>{timeout}s）")
        return 124, ""
    except Exception as e:
        print(f"[ERROR] 执行 ncmm {' '.join(args)} 失败: {e}")
        return 1, ""

def query_binary_version(binary_path):
    """读取二进制自身报告的版本号，仅用于日志与 VERSION 记录。"""
    code, output = run_ncmm_cli(binary_path, ["--version"], timeout=30, capture=True)
    if code != 0:
        return None
    match = re.search(r'Version:\s*([0-9][0-9A-Za-z\.\-]*)', output)
    if match:
        return match.group(1)
    return None

def update_with_ncmm(binary_path, binary_name):
    """
    手动触发 ncmm 更新：只调用 `ncmm update --apply`。
    检查、下载、镜像、解压、替换、配置升级全部由 ncmm 按自身规则完成。
    首次失败可能是运行中的进程占用了二进制，释放后重试一次。
    """
    code, _ = run_ncmm_cli(binary_path, ["update", "--apply"])
    if code == 0:
        return True

    print("[WARNING] ncmm update --apply 首次执行失败，可能是运行中的 ncmm 进程占用了二进制，")
    print("[WARNING] 正在尝试释放占用后重试一次...")
    stop_running_ncmm(binary_name)
    code, _ = run_ncmm_cli(binary_path, ["update", "--apply"])
    if code == 0:
        return True

    print(f"[ERROR] ncmm update --apply 执行失败（状态码 {code}），本次更新未完成。")
    return False

def cleanup_old_binary(binary_path):
    """ncmm 替换二进制时会留下 *.old，这里做一次尽力而为的清理。"""
    old_path = binary_path + ".old"
    if not os.path.exists(old_path):
        return
    try:
        os.remove(old_path)
        print(f"[LOG] 已清理旧版本备份文件: {old_path}")
    except Exception as e:
        print(f"[WARNING] 清理旧版本备份失败（下次 ncmm 启动时会自行清理）: {e}")

# 8. 首次安装（没有任何可调用的 ncmm 二进制时）：下载 release 安装包并落地
def install_from_archive(remote_tag, assets, binary_name, binary_path, config_path, version_file):
    os_part, arch_part, ext = get_platform_info()
    print(f"[LOG] 判定当前主机系统为: {os_part}, 架构为: {arch_part}, 下载格式为: {ext}")

    download_url = None
    asset_filename = f"ncmm_{os_part}_{arch_part}{ext}"

    if assets:
        for asset in assets:
            name = asset.get('name', '')
            if os_part.lower() in name.lower() and arch_part.lower() in name.lower() and name.endswith(ext):
                download_url = asset.get('browser_download_url')
                print(f"[LOG] 匹配到 release 资源: {name}")
                break

    if not download_url:
        print("[WARNING] GitHub API 资源匹配失败，尝试手动拼接下载链接...")
        download_url = f"https://github.com/3899/ncmm/releases/download/{remote_tag}/{asset_filename}"

    # 创建本地临时解压目录
    temp_dir = os.path.join(current_dir, "_temp_ncmm_update_")
    if os.path.exists(temp_dir):
        shutil.rmtree(temp_dir)
    os.makedirs(temp_dir, exist_ok=True)

    archive_tmp_path = os.path.join(temp_dir, asset_filename)
    headers = {
        'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36'
    }

    try:
        print(f"[LOG] 正在下载安装包到 {archive_tmp_path} ...")
        if not download_file_with_fallback(download_url, archive_tmp_path, headers=headers, timeout=45):
            print("[ERROR] 尝试所有镜像源与原地址均无法成功下载安装包，安装中止。")
            return False

        extract_dir = os.path.join(temp_dir, "extracted")
        os.makedirs(extract_dir, exist_ok=True)
        print(f"[LOG] 正在解压至 {extract_dir} ...")
        if ext == ".zip":
            with zipfile.ZipFile(archive_tmp_path, 'r') as zip_ref:
                zip_ref.extractall(extract_dir)
        else:
            with tarfile.open(archive_tmp_path, "r:gz") as tar_ref:
                try:
                    tar_ref.extractall(extract_dir, filter='data')
                except TypeError:
                    tar_ref.extractall(extract_dir)
        print("[LOG] 解压成功")

        # 定位二进制文件
        extracted_binary_path = os.path.join(extract_dir, binary_name)
        if not os.path.exists(extracted_binary_path):
            for root, dirs, files in os.walk(extract_dir):
                if binary_name in files:
                    extracted_binary_path = os.path.join(root, binary_name)
                    break

        if not os.path.exists(extracted_binary_path):
            print(f"[ERROR] 解压出的产物中找不到二进制文件: {binary_name}，安装中止。")
            return False

        # 查找 config.yaml 默认配置；本地已有配置时不覆盖（配置结构升级由 ncmm 启动时自行完成）
        default_config_path = os.path.join(extract_dir, "config.yaml")
        if not os.path.exists(default_config_path):
            for root, dirs, files in os.walk(extract_dir):
                if "config.yaml" in files:
                    default_config_path = os.path.join(root, "config.yaml")
                    break

        if not os.path.exists(default_config_path):
            # 兼容处理：如果解包产物中无 config.yaml，从 GitHub 仓库直接下载最新默认配置
            raw_config_url = f"https://raw.githubusercontent.com/3899/ncmm/{remote_tag}/config/config.yaml"
            print("[LOG] 解压缩产物中缺少 config.yaml，正在从 GitHub 下载默认配置文件备用...")
            default_config_path = os.path.join(temp_dir, "config.yaml")
            if not download_file_with_fallback(raw_config_url, default_config_path, headers=headers, timeout=15):
                print("[ERROR] 无法从 GitHub 获取默认配置文件，安装中止。")
                return False

        print("[LOG] 正在写入二进制文件...")
        shutil.copy2(extracted_binary_path, binary_path)

        if os.path.exists(config_path):
            print("[LOG] 本地已存在 config.yaml，保留用户配置不覆盖。")
        else:
            print("[LOG] 未发现本地 config.yaml，写入默认配置文件。")
            shutil.copy2(default_config_path, config_path)

        # 赋予执行权限
        if 'windows' not in platform.system().lower():
            try:
                os.chmod(binary_path, 0o755)
                print("[LOG] 成功为二进制文件授予 0755 执行权限。")
            except Exception as e:
                print(f"[WARNING] 为二进制授权失败: {e}，请稍后手动排查。")

        write_local_version(version_file, remote_tag)
        print(f"[SUCCESS] ncmm 已成功安装 {remote_tag} 版本！")
        return True

    except Exception as e:
        print(f"[ERROR] 安装流程发生严重异常: {e}")
        return False

    finally:
        # 清理临时文件
        if os.path.exists(temp_dir):
            try:
                shutil.rmtree(temp_dir)
                print("[LOG] 已清理临时文件目录。")
            except Exception as e:
                print(f"[WARNING] 清理临时目录失败: {e}")

def main():
    is_windows = 'windows' in platform.system().lower()
    binary_name = "ncmm.exe" if is_windows else "ncmm"
    binary_path = os.path.join(target_dir, binary_name)
    config_path = os.path.join(target_dir, "config.yaml")
    version_file = os.path.join(target_dir, "VERSION")

    # 没有可调用的二进制：只能由脚本完成首次安装（此时不存在"两套更新规则"的问题）
    if not os.path.exists(binary_path):
        print(f"[LOG] 未检测到 {binary_name}，判定为首次安装，由脚本下载 release 安装包...")
        remote_tag, assets = get_latest_release()
        if not remote_tag:
            print("[ERROR] 无法获取 GitHub 最新版本信息，安装中止。")
            sys.exit(1)
        print(f"[LOG] GitHub 最新版本为: {remote_tag}")
        ok = install_from_archive(remote_tag, assets, binary_name, binary_path, config_path, version_file)
        sys.exit(0 if ok else 1)

    # 已有程序：手动触发 ncmm 自身的更新规则，脚本不做任何版本判断
    print(f"[LOG] 当前 VERSION 记录: {read_local_version(version_file)}")
    current = query_binary_version(binary_path)
    if current:
        print(f"[LOG] 二进制当前版本: {current}")
    print("[LOG] 交由 ncmm 自身的更新规则执行（ncmm update --apply），脚本仅做手动触发...")

    if not update_with_ncmm(binary_path, binary_name):
        print("[ERROR] 更新失败，更新中止。")
        sys.exit(1)

    final_version = query_binary_version(binary_path)
    if final_version:
        write_local_version(version_file, final_version)
    cleanup_old_binary(binary_path)
    print(f"[SUCCESS] 更新流程完成，当前版本: {final_version or '未知'}")
    print("[LOG] 提示：如正在运行 ncmm web / 面板服务，请重启后使用新版本。")

if __name__ == '__main__':
    main()
