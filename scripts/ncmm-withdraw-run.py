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
    # 1. 如果命令行直接传入了参数，则直接透传 (例如: python ncmm-withdraw-run.py --target 15 --advance 60)
    if len(sys.argv) > 1:
        return sys.argv[1:]

    # 2. 从环境变量读取动态配置，未配置则使用推荐默认值
    # 青龙面板可在【环境变量】中随意添加/修改以下变量，无需改动脚本代码
    target = os.getenv("WITHDRAW_TARGET", "15,25,0.3,0.1").strip()
    snipe = os.getenv("WITHDRAW_SNIPE", "auto").strip()
    advance = os.getenv("WITHDRAW_ADVANCE", "50").strip()
    channel = os.getenv("WITHDRAW_CHANNEL", "ALIPAY").strip()
    fallback = os.getenv("WITHDRAW_FALLBACK", "true").strip().lower()
    account = os.getenv("WITHDRAW_ACCOUNT", "").strip()

    args = []
    if target:
        args.extend(["--target", target])
    if snipe:
        args.extend(["--snipe", snipe])
    if advance:
        args.extend(["--advance", advance])
    if channel:
        args.extend(["--channel", channel])
    if fallback == "false":
        args.append("--fallback=false")
    if account:
        args.extend(["--account", account])

    return args


# 获取当前脚本所在的真实目录
current_dir = os.path.dirname(os.path.abspath(__file__))

# 判断操作系统与二进制文件名
is_windows = 'windows' in platform.system().lower()
binary_name = "ncmm-withdraw.exe" if is_windows else "ncmm-withdraw"

# 自动扫描可执行文件路径
candidate_paths = [
    os.path.join(current_dir, binary_name),
    os.path.join(current_dir, "plugins", binary_name),
    os.path.join(current_dir, "plugins", "ncmm-withdraw", binary_name),
    os.path.join(os.path.dirname(current_dir), "plugins", binary_name),
    os.path.join(os.path.dirname(current_dir), "plugins", "ncmm-withdraw", binary_name),
    os.path.join(os.path.dirname(current_dir), binary_name),
]

binary_path = None
for p in candidate_paths:
    if os.path.isfile(p):
        binary_path = p
        break

if not binary_path:
    print(f"[ERROR] 未找到提现程序: {binary_name}")
    print(f"[ERROR] 搜索候选路径:")
    for p in candidate_paths:
        print(f"  - {p}")
    print(f"[INFO] 请先编译或放置 {binary_name} 至上述路径之一，并确保有可执行权限。")
    sys.exit(1)

# 确保 Linux/Unix 环境下具有可执行权限
if not is_windows and not os.access(binary_path, os.X_OK):
    try:
        os.chmod(binary_path, 0o755)
    except Exception as e:
        print(f"[WARN] 赋予执行权限失败: {e}")

run_args = get_run_args()
cmd = [binary_path] + run_args

print("=================================================================")
print(f"[INFO] 提现可执行程序: {binary_path}")
print(f"[INFO] 启动参数: {' '.join(run_args)}")
print(f"[INFO] 进程生命周期: 单次抢购完成后自动停止退出，不常驻后台。")
print("=================================================================")

result = subprocess.run(cmd, cwd=os.path.dirname(binary_path))
sys.exit(result.returncode)
