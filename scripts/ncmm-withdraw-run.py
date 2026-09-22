#!/usr/bin/env python3
# -*- coding: utf-8 -*-

"""
cron: 30 59 7,11,17,19,23 * * *
new Env('NCMM 云贝提现抢额度')
"""

import os
import sys
import platform
import subprocess

# ==============================================================================
#  NCMM 云贝提现抢额度启动脚本
#
#  【运行机制与生命周期说明】
#  1. 本脚本绝不会常驻后台死循环挂起！
#  2. 触发后，程序自动识别当前最临近的官方放量时间点 (0:00, 8:00, 12:00, 18:00, 20:00)；
#  3. 倒计时精准休眠到目标时间点前 (默认提前 50ms) 发起提现请求；
#  4. 提交提现、获取结果、发送通知后，程序立即以 Exit Code 0 自动正常退出（耗时约 31 秒）；
#  5. 等待青龙或系统的下一次定时触发，绝不占用系统后台内存。
#
#  【定时设置建议】
#  - 青龙 6 位 Cron (支持秒): 30 59 7,11,17,19,23 * * * (每个放量点前 30 秒触发)
#  - Linux 5 位 Cron (分钟级): 59 7,11,17,19,23 * * * (每个放量点前 1 分钟触发)
# ==============================================================================

def extract_accounts_from_config(cfg_path):
    """从 config.yaml 解析出主账号与辅助账号列表"""
    accounts = []
    try:
        import yaml
        with open(cfg_path, "r", encoding="utf-8") as f:
            cfg = yaml.safe_load(f)
            if isinstance(cfg, dict):
                acc = cfg.get("accounts", {})
                if isinstance(acc, dict):
                    main = acc.get("main") or acc.get("primary")
                    if main:
                        accounts.append(str(main).strip())
                    secs = acc.get("secondary")
                    if isinstance(secs, list):
                        for s in secs:
                            if s:
                                accounts.append(str(s).strip())
    except Exception:
        pass

    if not accounts:
        import re
        try:
            with open(cfg_path, "r", encoding="utf-8") as f:
                text = f.read()
                m_main = re.search(r'^\s*(?:main|primary):\s*["\']?([^"\'\r\n#]+)', text, re.MULTILINE)
                if m_main:
                    accounts.append(m_main.group(1).strip())
                m_secs = re.findall(r'^\s*-\s*["\']?([^"\'\r\n#]+\.json)', text, re.MULTILINE)
                for s in m_secs:
                    if s.strip() and s.strip() not in accounts:
                        accounts.append(s.strip())
        except Exception:
            pass

    return accounts


def find_account_path(work_dir):
    """
    在执行工作目录或执行工作目录/run 下自动查找账号 Cookie 文件：
    1. 优先从 config.yaml 提取账号路径，并在工作目录及 run 子目录下定位实际文件；
    2. 若未配置，则依次扫描工作目录及 run 下常见的 Cookie JSON 文件 (如 9082.json, cookie.json, fan1.json 等)。
    """
    search_dirs = [work_dir, os.path.join(work_dir, "run")]

    # 1. 优先从 config.yaml 查找
    for d in search_dirs:
        for cfg_name in ["config.yaml", os.path.join("config", "config.yaml")]:
            cfg_file = os.path.join(d, cfg_name)
            if os.path.isfile(cfg_file):
                raw_accounts = extract_accounts_from_config(cfg_file)
                found_accs = []
                for acc in raw_accounts:
                    candidates = [
                        acc if os.path.isabs(acc) else None,
                        os.path.join(work_dir, os.path.basename(acc)),
                        os.path.join(work_dir, acc.lstrip("./")),
                        os.path.join(work_dir, "run", os.path.basename(acc)),
                        os.path.join(work_dir, "run", acc.lstrip("./")),
                        os.path.join(os.path.dirname(cfg_file), acc.lstrip("./")),
                    ]
                    for c in candidates:
                        if c and os.path.isfile(c):
                            abs_c = os.path.abspath(c)
                            if abs_c not in found_accs:
                                found_accs.append(abs_c)
                            break
                if found_accs:
                    return ",".join(found_accs)

    # 2. 直接扫描 Cookie JSON 文件
    for d in search_dirs:
        if not os.path.isdir(d):
            continue
        for name in ["cookie.json", os.path.join("config", "cookie.json"), os.path.join("temp", "cookie.json")]:
            p = os.path.join(d, name)
            if os.path.isfile(p):
                return os.path.abspath(p)

        try:
            for fname in os.listdir(d):
                if fname.endswith(".json") and fname.lower() not in (
                    "package.json", "plugin.json", "tsconfig.json", "package-lock.json"
                ):
                    p = os.path.join(d, fname)
                    if os.path.isfile(p):
                        return os.path.abspath(p)
        except Exception:
            pass

    return ""


def find_notify_path(work_dir):
    """
    在执行工作目录或执行工作目录/run 下自动查找 notify.yaml：
    1. 优先检索执行工作目录 (work_dir) 及执行工作目录/run (work_dir/run)；
    2. 优先选择实际启用了推送通道 (enabled: true) 的配置文件，避开空的默认模板。
    """
    search_dirs = [work_dir, os.path.join(work_dir, "run")]
    candidate_subpaths = ["notify.yaml", os.path.join("config", "notify.yaml")]

    found_files = []
    for d in search_dirs:
        for sub in candidate_subpaths:
            p = os.path.abspath(os.path.join(d, sub))
            if os.path.isfile(p) and p not in found_files:
                found_files.append(p)

    if not found_files:
        return ""

    # 优先查找是否有已启用推送渠道的文件 (包含 enabled: true)
    for p in found_files:
        try:
            with open(p, "r", encoding="utf-8") as f:
                content = f.read().lower()
                if "enabled: true" in content or "enabled:true" in content:
                    return p
        except Exception:
            pass

    # 避免默认选中 config/notify.yaml 禁用模板
    for p in found_files:
        if not p.endswith(os.path.join("config", "notify.yaml")):
            return p

    return found_files[0]


def find_config_path(work_dir):
    """在执行工作目录或执行工作目录/run 下自动查找 config.yaml"""
    search_dirs = [work_dir, os.path.join(work_dir, "run")]
    for d in search_dirs:
        for name in ["config.yaml", os.path.join("config", "config.yaml")]:
            p = os.path.join(d, name)
            if os.path.isfile(p):
                return os.path.abspath(p)
    return ""


def get_run_args(work_dir):
    """
    智能合并命令行参数与环境变量/自动检测路径：
    1. 若命令行传入了 --help 或 -h，直接透传展示帮助。
    2. 命令行显式传入的参数具有最高优先级。
    3. 未指定账号或 notify.yaml 时，自动在“执行工作目录”或“执行工作目录/run”下检索，
       而不是在插件安装目录 plugins/ncmm-withdraw 中查找。
    4. 自动补全整点秒杀核心参数 (--snipe auto, --advance 50 等)。
    """
    raw_args = sys.argv[1:]

    # 如果包含帮助参数，直接透传
    if any(arg in ("--help", "-help", "-h", "--h") for arg in raw_args):
        return raw_args

    def has_flag(flag_names):
        for arg in raw_args:
            for name in flag_names:
                if arg == name or arg.startswith(name + "="):
                    return True
        return False

    merged_args = list(raw_args)

    # 1. 抢购时段 (snipe)
    if not has_flag(["--snipe", "-snipe"]):
        snipe = os.getenv("WITHDRAW_SNIPE", "auto").strip()
        if snipe:
            merged_args.extend(["--snipe", snipe])

    # 2. 提前请求毫秒数 (advance)
    if not has_flag(["--advance", "-advance"]):
        advance = os.getenv("WITHDRAW_ADVANCE", "50").strip()
        if advance:
            merged_args.extend(["--advance", advance])

    # 3. 目标金额 (target)
    if not has_flag(["--target", "-target"]):
        target = os.getenv("WITHDRAW_TARGET", "15,25,0.3,0.1").strip()
        if target:
            merged_args.extend(["--target", target])

    # 4. 提现渠道 (channel)
    if not has_flag(["--channel", "-channel"]):
        channel = os.getenv("WITHDRAW_CHANNEL", "ALIPAY").strip()
        if channel:
            merged_args.extend(["--channel", channel])

    # 5. 自动降级 (fallback)
    if not has_flag(["--fallback", "-fallback"]):
        fallback = os.getenv("WITHDRAW_FALLBACK", "true").strip().lower()
        if fallback == "false":
            merged_args.append("--fallback=false")

    # 6. 指定账号 (account)
    if not has_flag(["--account", "-account", "--cookie", "-cookie", "-a"]):
        account = os.getenv("WITHDRAW_ACCOUNT", "").strip()
        if not account and work_dir:
            account = find_account_path(work_dir)
        if account:
            merged_args.extend(["--account", account])

    # 7. 通知配置文件 (notify)
    if not has_flag(["--notify", "-notify"]):
        notify = os.getenv("WITHDRAW_NOTIFY", "").strip()
        if not notify and work_dir:
            notify = find_notify_path(work_dir)
        if notify:
            merged_args.extend(["--notify", notify])

    # 8. 核心配置文件 (config)
    if not has_flag(["--config", "-config"]):
        cfg = os.getenv("WITHDRAW_CONFIG", "").strip()
        if not cfg and work_dir:
            cfg = find_config_path(work_dir)
        if cfg:
            merged_args.extend(["--config", cfg])

    return merged_args


# 获取当前脚本所在的真实目录
current_dir = os.path.dirname(os.path.abspath(__file__))
parent_dir = os.path.dirname(current_dir)

# 判断操作系统与二进制文件名
is_windows = 'windows' in platform.system().lower()
binary_name = "ncmm-withdraw.exe" if is_windows else "ncmm-withdraw"

# 自动扫描可执行文件路径 (多路径容错与兼容)
candidate_paths = [
    os.path.join(current_dir, binary_name),
    os.path.join(current_dir, "plugins", binary_name),
    os.path.join(current_dir, "plugins", "ncmm-withdraw", binary_name),
    os.path.join(current_dir, "plugins", "ncmm-withdraw-run", binary_name),
    os.path.join(parent_dir, "plugins", binary_name),
    os.path.join(parent_dir, "plugins", "ncmm-withdraw", binary_name),
    os.path.join(parent_dir, "plugins", "ncmm-withdraw-run", binary_name),
    os.path.join(parent_dir, binary_name),
]

binary_path = None
for p in candidate_paths:
    if os.path.isfile(p):
        binary_path = p
        break

if not binary_path:
    print(f"[ERROR] 未找到提现程序: {binary_name}")
    print(f"[ERROR] 搜索候选路径:")
    seen_paths = set()
    for p in candidate_paths:
        if p not in seen_paths:
            seen_paths.add(p)
            print(f"  - {p}")
    print(f"[INFO] 请先编译或放置 {binary_name} 至上述路径之一，并确保有可执行权限。")
    sys.exit(1)

# 确保 Linux/Unix 环境下具有可执行权限
if not is_windows and not os.access(binary_path, os.X_OK):
    try:
        os.chmod(binary_path, 0o755)
    except Exception as e:
        print(f"[WARN] 赋予执行权限失败: {e}")

# 智能确定运行工作目录（优先 ncmm 项目根目录，确保相对路径配置文件与 Cookie 能被正确找到）
work_dir = current_dir
if (os.path.isfile(os.path.join(current_dir, "config.yaml")) or 
    os.path.isdir(os.path.join(current_dir, "plugins")) or
    os.path.isdir(os.path.join(current_dir, "run"))):
    work_dir = current_dir
elif (os.path.isfile(os.path.join(parent_dir, "config.yaml")) or 
      os.path.isdir(os.path.join(parent_dir, "plugins")) or
      os.path.isdir(os.path.join(parent_dir, "run"))):
    work_dir = parent_dir
else:
    work_dir = os.path.dirname(binary_path)

run_args = get_run_args(work_dir)
cmd = [binary_path] + run_args

print("=================================================================")
print(f"[INFO] 提现可执行程序: {binary_path}")
print(f"[INFO] 执行工作目录: {work_dir}")
print(f"[INFO] 启动参数: {' '.join(run_args)}")
print(f"[INFO] 进程生命周期: 单次抢购完成后自动停止退出，不常驻后台。")
print("=================================================================")

result = subprocess.run(cmd, cwd=work_dir)
sys.exit(result.returncode)
