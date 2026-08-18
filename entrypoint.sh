#!/bin/sh

set -e

# 1. 首次启动检测并释放配置文件到挂载目录
if [ ! -f /data/config.yaml ]; then
  echo "Initializing default config.yaml..."
  mkdir -p /data
  cp /etc/ncmm/config.yaml /data/config.yaml
fi

if [ ! -f /data/notify.yaml ]; then
  echo "Initializing default notify.yaml..."
  mkdir -p /data
  cp /etc/ncmm/notify.yaml /data/notify.yaml
fi

sync_cookiecloud() {
  if [ "$COOKIECLOUD_UUID" != "your-uuid" ] && [ -n "$COOKIECLOUD_UUID" ]; then
    echo "Syncing cookies from CookieCloud..."
    ncmm -c /data/config.yaml login cookiecloud \
      -u "$COOKIECLOUD_UUID" \
      -p "$COOKIECLOUD_PASSWORD" \
      -s "$COOKIECLOUD_SERVER" -m || echo "Warning: CookieCloud sync failed, will proceed with existing cookies."
  fi
}

# Cron jobs use this branch so they do not rebuild the active crontab while
# crond is running.
if [ "$1" = "__run" ]; then
  shift
  sync_cookiecloud
  exec ncmm -c /data/config.yaml "$@"
fi

# WebUI uses the Go scheduler. CRON_* variables remain visible to ncmm and are
# imported as read-only schedules for backward compatibility.
if [ "$1" = "web" ]; then
  sync_cookiecloud
  exec ncmm -c /data/config.yaml "$@"
fi

# 2. 解析环境变量中的多重定时任务
# 清空已有的 crontab，防止重复追加
true > /etc/crontabs/root

# 导出关键环境变量以供 cron 任务调用
mkdir -p /etc/ncmm
printenv | grep -E '^(COOKIECLOUD_|NCMM_|TZ=|PATH=)' | while IFS= read -r env_line; do
  key=${env_line%%=*}
  value=${env_line#*=}
  escaped=$(printf '%s' "$value" | sed "s/'/'\\\\''/g")
  printf "export %s='%s'\n" "$key" "$escaped"
done > /etc/ncmm/env.sh
chmod 600 /etc/ncmm/env.sh

# 方案 A：从 command 块传入定时任务 ($1 = "cron" 模式)
if [ "$1" = "cron" ]; then
  echo "Cron mode detected (via command arguments). Parsing schedules..."
  echo "$2" | while read -r line; do
    [ -z "$line" ] || echo "$line" | grep -q '^[[:space:]]*#' && continue
    read -r m h dom mon dow cmd <<EOF
$line
EOF
    if [ -n "$m" ] && [ -n "$h" ] && [ -n "$dom" ] && [ -n "$mon" ] && [ -n "$dow" ] && [ -n "$cmd" ]; then
      cron_expr="$m $h $dom $mon $dow"
      echo "$cron_expr . /etc/ncmm/env.sh && /entrypoint.sh __run $cmd > /proc/1/fd/1 2>&1" >> /etc/crontabs/root
      echo "Added cron job: $cron_expr -> ncmm $cmd"
    fi
  done
fi

# 方案 B：从以 CRON 开头的环境变量中解析定时任务
env | grep -E '^CRON' | while read -r env_var; do
  value=$(echo "$env_var" | cut -d'=' -f2-)
  [ -z "$value" ] && continue

  read -r m h dom mon dow cmd <<EOF
$value
EOF
  if [ -n "$m" ] && [ -n "$h" ] && [ -n "$dom" ] && [ -n "$mon" ] && [ -n "$dow" ] && [ -n "$cmd" ]; then
    cron_expr="$m $h $dom $mon $dow"
    # 避免和 Command 里的规则重复
    if ! grep -qF "$cron_expr . /etc/ncmm/env.sh && /entrypoint.sh __run $cmd" /etc/crontabs/root; then
      echo "$cron_expr . /etc/ncmm/env.sh && /entrypoint.sh __run $cmd > /proc/1/fd/1 2>&1" >> /etc/crontabs/root
      echo "Added cron job (env): $cron_expr -> ncmm $cmd"
    fi
  else
    if [ -n "$m" ] && [ -n "$h" ] && [ -n "$dom" ] && [ -n "$mon" ] && [ -n "$dow" ]; then
      cron_expr="$m $h $dom $mon $dow"
      if ! grep -qF "$cron_expr . /etc/ncmm/env.sh && /entrypoint.sh __run task" /etc/crontabs/root; then
        echo "$cron_expr . /etc/ncmm/env.sh && /entrypoint.sh __run task > /proc/1/fd/1 2>&1" >> /etc/crontabs/root
        echo "Added cron job (env): $cron_expr -> ncmm task"
      fi
    fi
  fi
done

# 如果生成了有效的 crontab 规则，启动 crond 守护进程
if [ -s /etc/crontabs/root ]; then
  echo "Starting crond daemon..."
  exec crond -f -l 2
fi

# 3. 如果未配置任何定时任务，则是单次运行模式：同步 Cookie 并执行传入命令
if [ "$1" = "cron" ]; then
  echo "Error: Cron mode requested but no valid schedules found."
  exit 1
fi

sync_cookiecloud

exec ncmm -c /data/config.yaml "$@"
