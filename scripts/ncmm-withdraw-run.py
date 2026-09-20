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

def get_run_args():
    """
    智能合并命令行参数与环境变量/默认值：
    1. 若命令行传入了 --help 或 -h，直接透传展示帮助。
    2. 命令行显式传入的参数具有最高优先级。
    3. 未在命令行传入的参数，将自动回退读取环境变量或使用推荐默认值（如 --snipe auto, --advance 50 等），
       彻底避免因仅传入 --account 而丢失整点秒杀的核心逻辑。
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
        if account:
            merged_args.extend(["--account", account])

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
if os.path.isfile(os.path.join(current_dir, "config.yaml")) or os.path.isdir(os.path.join(current_dir, "plugins")):
    work_dir = current_dir
elif os.path.isfile(os.path.join(parent_dir, "config.yaml")) or os.path.isdir(os.path.join(parent_dir, "plugins")):
    work_dir = parent_dir
else:
    work_dir = os.path.dirname(binary_path)

run_args = get_run_args()
cmd = [binary_path] + run_args

print("=================================================================")
print(f"[INFO] 提现可执行程序: {binary_path}")
print(f"[INFO] 执行工作目录: {work_dir}")
print(f"[INFO] 启动参数: {' '.join(run_args)}")
print(f"[INFO] 进程生命周期: 单次抢购完成后自动停止退出，不常驻后台。")
print("=================================================================")

result = subprocess.run(cmd, cwd=work_dir)
sys.exit(result.returncode)
