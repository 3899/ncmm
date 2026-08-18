#!/usr/bin/env python3
# -*- coding: utf-8 -*-

"""
cron: 1 1 1 1 1
new Env('NCMM 安装、更新')
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

# 用户配置中这些键包含动态映射或完整列表，更新时必须整体保留。
ATOMIC_BLOCK_KEYS = {
    'antiCheatTokens', 'topics', 'fast_tasks', 'slow_tasks',
    'proxy_mirrors', 'idsFile', 'titlesFile', 'messagesFile', 'imageUrls'
}

# 1. 稳定获取当前脚本所在的真实目录
current_dir = os.path.dirname(os.path.abspath(__file__))
target_dir = current_dir  # 直接使用脚本所在目录作为目标目录

print(f"[LOG] Python脚本所在目录: {current_dir}")
print(f"[LOG] 目标二进制目录: {target_dir}")

# 确保目标目录存在（实际上已经存在，不需要额外创建）
# os.makedirs(target_dir, exist_ok=True)

# 2. 识别系统平台与架构判定逻辑
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

# 3. 版本号解析逻辑（用于大小对比）
def parse_version(ver_str):
    ver_str = ver_str.strip().lstrip('vV')
    parts = []
    for p in ver_str.split('.'):
        digits = ""
        for char in p:
            if char.isdigit():
                digits += char
            else:
                break
        if digits:
            parts.append(int(digits))
        else:
            parts.append(0)
    return tuple(parts)

# 4. 获取 GitHub 最新 Release 标签与资产列表
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

# 5. 进程查杀释放逻辑 (释放被占用的二进制文件)
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

# 6. YAML 解析与合并模块
def parse_yaml(content):
    """将 YAML 文本解析为结构化行记录列表，每条记录包含缩进、类型、键路径等信息。"""
    lines = content.splitlines()
    parsed_lines = []
    stack = []

    for i, raw_line in enumerate(lines):
        indent = len(raw_line) - len(raw_line.lstrip(' '))
        stripped = raw_line.strip()

        line_type = 'empty'
        key = None
        val = None

        if not stripped:
            line_type = 'empty'
        elif stripped.startswith('#'):
            line_type = 'comment'
        elif stripped.startswith('-'):
            line_type = 'list_item'
            # 处理 - key: val 形式
            after_dash = stripped[1:].strip()
            if ':' in after_dash:
                parts = after_dash.split(':', 1)
                key = parts[0].strip()
                val = parts[1].strip()
        elif ':' in stripped:
            parts = stripped.split(':', 1)
            key = parts[0].strip()
            val = parts[1].strip()
            line_type = 'key'
        else:
            line_type = 'comment'

        parsed_lines.append({
            'index': i,
            'raw': raw_line,
            'indent': indent,
            'stripped': stripped,
            'type': line_type,
            'key': key,
            'val': val,
            'key_path': None
        })

    for line in parsed_lines:
        indent = line['indent']
        if line['type'] == 'key':
            while stack and stack[-1][0] >= indent:
                stack.pop()
            line['key_path'] = tuple([s[1] for s in stack] + [line['key']])
            stack.append((indent, line['key']))
        elif line['type'] == 'list_item':
            line['key_path'] = tuple([s[1] for s in stack])

    return parsed_lines

def get_block_by_key_name(parsed_lines, target_key):
    """根据键名获取完整块（适用于 antiCheatTokens, topics 等特异性动态 Block 键）。"""
    key_idx = -1
    for i, line in enumerate(parsed_lines):
        if line['type'] == 'key' and line['key'] == target_key:
            key_idx = i
            break
    if key_idx == -1:
        return None, -1

    key_line = parsed_lines[key_idx]
    d = key_line['indent']
    block_lines = [key_line['raw']]
    for j in range(key_idx + 1, len(parsed_lines)):
        line = parsed_lines[j]
        if line['type'] in ('empty', 'comment'):
            has_child = False
            for k in range(j + 1, len(parsed_lines)):
                fut = parsed_lines[k]
                if fut['type'] not in ('empty', 'comment'):
                    has_child = fut['indent'] > d
                    break
            if has_child:
                block_lines.append(line['raw'])
            continue
        if line['indent'] <= d:
            break
        block_lines.append(line['raw'])
    return block_lines, d

def get_data_paths(parsed_lines):
    """获取所有叶子键路径（即有值的键，不包含纯容器键）。"""
    all_paths = set()
    for line in parsed_lines:
        if line['type'] == 'key':
            all_paths.add(line['key_path'])
    data_paths = set()
    for p in all_paths:
        is_container = False
        for other in all_paths:
            if len(other) > len(p) and other[:len(p)] == p:
                is_container = True
                break
        if not is_container:
            data_paths.add(p)
    return data_paths

def get_block_for_path(parsed_lines, path):
    """提取指定路径的键行及其所有子内容行（值、列表项、嵌套内容）。"""
    key_idx = -1
    for i, line in enumerate(parsed_lines):
        if line['type'] == 'key' and line['key_path'] == path:
            key_idx = i
            break
    if key_idx == -1:
        return None

    key_line = parsed_lines[key_idx]
    d = key_line['indent']

    block_lines = [key_line['raw']]
    for j in range(key_idx + 1, len(parsed_lines)):
        line = parsed_lines[j]
        if line['type'] in ('empty', 'comment'):
            has_child_after = False
            for k in range(j + 1, len(parsed_lines)):
                future = parsed_lines[k]
                if future['type'] not in ('empty', 'comment'):
                    has_child_after = future['indent'] > d
                    break
            if has_child_after:
                block_lines.append(line['raw'])
            continue
        if line['indent'] <= d:
            break
        block_lines.append(line['raw'])

    return block_lines

def adjust_block_indent(block_lines, target_indent, source_indent):
    """将一组行的缩进从 source_indent 基准调整到 target_indent 基准。"""
    delta = target_indent - source_indent
    if delta == 0:
        return block_lines
    adjusted = []
    for raw in block_lines:
        if not raw.strip():
            adjusted.append(raw)
            continue
        current = len(raw) - len(raw.lstrip(' '))
        new_indent = max(0, current + delta)
        adjusted.append(' ' * new_indent + raw.lstrip(' '))
    return adjusted

def merge_yaml(default_content, user_content):
    """将用户配置值合并到默认配置结构中，保持默认配置的缩进风格。"""
    # 方案 1：若环境安装了 PyYAML，优先使用安全的 AST 级深层字典合并
    try:
        import yaml
        def deep_merge(def_dict, user_dict):
            merged = dict(def_dict)
            for k, u_val in user_dict.items():
                if k in merged:
                    if k in ATOMIC_BLOCK_KEYS:
                        merged[k] = u_val
                    elif isinstance(merged[k], dict) and isinstance(u_val, dict):
                        merged[k] = deep_merge(merged[k], u_val)
                    else:
                        merged[k] = u_val
                else:
                    merged[k] = u_val
            return merged

        def_obj = yaml.safe_load(default_content)
        user_obj = yaml.safe_load(user_content)
        if isinstance(def_obj, dict) and isinstance(user_obj, dict):
            merged_obj = deep_merge(def_obj, user_obj)
            return yaml.dump(merged_obj, allow_unicode=True, sort_keys=False, indent=2)
    except Exception:
        pass

    # 方案 2：纯 Python 文本级解析与原子 Block 保护合并
    default_lines = parse_yaml(default_content)
    user_lines = parse_yaml(user_content)

    user_data_paths = get_data_paths(user_lines)
    default_data_paths = get_data_paths(default_lines)

    output = []
    skip_depth = -1  # 当前正在跳过的键的缩进层级

    for line in default_lines:
        if skip_depth != -1:
            if line['type'] in ('empty', 'comment'):
                continue
            if line['indent'] > skip_depth:
                continue
            skip_depth = -1

        if line['type'] in ('comment', 'empty'):
            output.append(line['raw'])
            continue

        if line['type'] == 'list_item':
            continue

        if line['type'] == 'key':
            key_name = line['key']
            path = line['key_path']

            # 处理原子 Block 键 (如 antiCheatTokens, topics)
            if key_name in ATOMIC_BLOCK_KEYS:
                user_blk, user_d = get_block_by_key_name(user_lines, key_name)
                if user_blk:
                    adjusted = adjust_block_indent(user_blk, line['indent'], user_d)
                    output.extend(adjusted)
                else:
                    def_blk, def_d = get_block_by_key_name(default_lines, key_name)
                    if def_blk:
                        output.extend(def_blk)
                    else:
                        output.append(line['raw'])
                skip_depth = line['indent']
                continue

            # 容器键（非叶子）：直接输出默认模板的行
            if path not in default_data_paths:
                output.append(line['raw'])
                continue

            # 叶子数据键：优先使用用户值，否则使用默认值
            if path in user_data_paths:
                user_block = get_block_for_path(user_lines, path)
                if user_block:
                    user_key_indent = len(user_block[0]) - len(user_block[0].lstrip(' '))
                    adjusted = adjust_block_indent(user_block, line['indent'], user_key_indent)
                    output.extend(adjusted)
                else:
                    output.append(line['raw'])
            else:
                default_block = get_block_for_path(default_lines, path)
                if default_block:
                    output.extend(default_block)
                else:
                    output.append(line['raw'])

            skip_depth = line['indent']

    return '\n'.join(output) + '\n'

# 7. 带有多镜像重试与原地址兜底的下载模块
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

def main():
    is_windows = 'windows' in platform.system().lower()
    version_file = os.path.join(target_dir, "VERSION")
    binary_name = "ncmm.exe" if is_windows else "ncmm"
    binary_path = os.path.join(target_dir, binary_name)
    config_path = os.path.join(target_dir, "config.yaml")
    
    # 读取本地版本
    local_version = "0.0.0"
    if os.path.exists(version_file):
        try:
            with open(version_file, 'r', encoding='utf-8') as f:
                local_version = f.read().strip()
        except Exception as e:
            print(f"[WARNING] 无法读取本地 VERSION 文件: {e}")
            
    print(f"[LOG] 当前本地版本为: {local_version}")
    
    # 获取 GitHub 最新版本
    remote_tag, assets = get_latest_release()
    if not remote_tag:
        print("[ERROR] 无法获取 GitHub 最新版本信息，更新中止。")
        sys.exit(1)
        
    print(f"[LOG] GitHub 最新版本为: {remote_tag}")
    
    # 对比版本
    local_v_parsed = parse_version(local_version)
    remote_v_parsed = parse_version(remote_tag)
    
    if remote_v_parsed <= local_v_parsed:
        print(f"[LOG] 当前版本 {local_version} 已是最新，无需更新。")
        sys.exit(0)
        
    print(f"[LOG] 检测到新版本 {remote_tag}，准备开始自动升级流程...")
    
    # 解析平台架构与后缀
    os_part, arch_part, ext = get_platform_info()
    print(f"[LOG] 判定当前主机系统为: {os_part}, 架构为: {arch_part}, 下载格式为: {ext}")
    
    # 查找匹配的下载 URL
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
        
    # 强制终止有可能占用的进程
    stop_running_ncmm(binary_name)
    
    # 如果二进制文件存在且正在被占用，先尝试删除
    if os.path.exists(binary_path):
        try:
            os.remove(binary_path)
            print(f"[LOG] 已删除旧的二进制文件: {binary_path}")
        except Exception as e:
            print(f"[WARNING] 无法删除旧的二进制文件: {e}，可能被占用，尝试继续...")
    
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
        # 下载压缩包
        print(f"[LOG] 正在下载升级包到 {archive_tmp_path} ...")
        if not download_file_with_fallback(download_url, archive_tmp_path, headers=headers, timeout=45):
            print("[ERROR] 尝试所有镜像源与原地址均无法成功下载升级包，更新中止。")
            sys.exit(1)
            
        # 解压缩
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
        
        # 定位二进制文件与默认配置文件
        extracted_binary_path = os.path.join(extract_dir, binary_name)
        if not os.path.exists(extracted_binary_path):
            # 兼容有可能多一层目录解压的情况
            for root, dirs, files in os.walk(extract_dir):
                if binary_name in files:
                    extracted_binary_path = os.path.join(root, binary_name)
                    break
                    
        if not os.path.exists(extracted_binary_path):
            print(f"[ERROR] 解压出的产物中找不到二进制文件: {binary_name}，更新中止。")
            sys.exit(1)
            
        # 查找 config.yaml 默认配置
        default_config_path = os.path.join(extract_dir, "config.yaml")
        if not os.path.exists(default_config_path):
            for root, dirs, files in os.walk(extract_dir):
                if "config.yaml" in files:
                    default_config_path = os.path.join(root, "config.yaml")
                    break
                    
        # 兼容处理：如果解包产物中无 config.yaml，从 GitHub 仓库直接下载最新默认配置
        if not os.path.exists(default_config_path):
            raw_config_url = f"https://raw.githubusercontent.com/3899/ncmm/{remote_tag}/config/config.yaml"
            print(f"[LOG] 解压缩产物中缺少 config.yaml，正在从 GitHub 下载默认配置文件备用...")
            default_config_path = os.path.join(temp_dir, "config.yaml")
            if not download_file_with_fallback(raw_config_url, default_config_path, headers=headers, timeout=15):
                print("[ERROR] 无法从 GitHub 获取默认配置文件，更新中止。")
                sys.exit(1)
                
        # 处理配置文件合并逻辑
        if os.path.exists(config_path):
            print("[LOG] 检测到本地已存在旧版 config.yaml，启动结构化差异对比与非破坏性合并...")
            try:
                with open(default_config_path, 'r', encoding='utf-8') as f:
                    default_yaml_content = f.read()
                with open(config_path, 'r', encoding='utf-8') as f:
                    user_yaml_content = f.read()
                    
                merged_yaml_content = merge_yaml(default_yaml_content, user_yaml_content)
                
                # 写入合并后的配置到临时路径，稍后进行替换
                temp_merged_config_path = os.path.join(temp_dir, "merged_config.yaml")
                with open(temp_merged_config_path, 'w', encoding='utf-8') as f:
                    f.write(merged_yaml_content)
                    
                print("[LOG] 差异合并成功完成。")
                final_config_source = temp_merged_config_path
            except Exception as e:
                print(f"[ERROR] 配置文件合并发生异常: {e}，为保证安全，本次更新中止。")
                sys.exit(1)
        else:
            print("[LOG] 本地未发现旧版 config.yaml，将直接采用最新默认配置文件。")
            final_config_source = default_config_path
            
        # 8. 拷贝/覆盖替换主程序、配置文件及 VERSION 文件
        print("[LOG] 正在写入最新二进制文件...")
        shutil.copy2(extracted_binary_path, binary_path)
        
        print("[LOG] 正在写入配置文件...")
        shutil.copy2(final_config_source, config_path)
        
        # 赋予执行权限
        if not is_windows:
            try:
                os.chmod(binary_path, 0o755)
                print("[LOG] 成功为二进制文件授予 0755 执行权限。")
            except Exception as e:
                print(f"[WARNING] 为二进制授权失败: {e}，请稍后手动排查。")
                
        # 写入新的版本号到 VERSION 文件
        print(f"[LOG] 正在写入 VERSION 文件...")
        with open(version_file, 'w', encoding='utf-8') as f:
            f.write(remote_tag.lstrip('vV') + '\n')
            
        print(f"[SUCCESS] ncmm 已成功升级至 {remote_tag} 版本！")
        
    except Exception as e:
        print(f"[ERROR] 升级流程发生严重异常: {e}")
        sys.exit(1)
        
    finally:
        # 清理临时文件
        if os.path.exists(temp_dir):
            try:
                shutil.rmtree(temp_dir)
                print("[LOG] 已清理临时升级文件目录。")
            except Exception as e:
                print(f"[WARNING] 清理临时目录失败: {e}")

if __name__ == '__main__':
    main()
