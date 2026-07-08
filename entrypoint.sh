#!/bin/sh -e

# ===== 防cron自举死循环（保留之前的修复）=====
if [ -n "$1" ] && [ "$1" != "cron" ]; then
  exec ncmm -c /data/config.yaml "$@"
fi

# 1. 首次启动释放配置文件
if [ ! -f /data/config.yaml ]; then
  echo "Initializing default config.yaml..."
  mkdir -p /data
  cp /etc/ncmm/config.yaml /data/config.yaml
fi

# ===== 【核心修改点1】先把CookieCloud同步挪到最前面，不管有没有定时任务都先同步 =====
if [ "$COOKIECLOUD_UUID" != "your-uuid" ] && [ -n "$COOKIECLOUD_UUID" ]; then
  echo "Syncing cookies from CookieCloud..."
  # 同步失败也不退出，避免定时任务完全无法启动，只打警告
  ncmm -c /data/config.yaml login cookiecloud \
    -u "$COOKIECLOUD_UUID" \
    -p "$COOKIECLOUD_PASSWORD" \
    -s "$COOKIECLOUD_SERVER" -m || echo "Warning: CookieCloud sync failed, will proceed with existing cookies."
fi

# 2. 解析环境变量中的多重定时任务
: > /etc/crontabs/root
mkdir -p /etc/ncmm
printenv | grep -E '^(COOKIECLOUD_|TZ|PATH=)' | sed 's/^/export /' > /etc/ncmm/env.sh

# 方案A：从command块传入定时任务
if [ "$1" = "cron" ]; then
  echo "Cron mode detected (via command arguments). Parsing schedules..."
  echo "$2" | while read -r line; do
    [ -z "$line" ] || echo "$line" | grep -q '^[[:space:]]*#' && continue
    read -r m h dom mon dow cmd <<EOF
$line
EOF
    if [ -n "$m" ] && [ -n "$h" ] && [ -n "$dom" ] && [ -n "$mon" ] && [ -n "$dow" ] && [ -n "$cmd" ]; then
      cron_expr="$m $h $dom $mon $dow"
      echo "$cron_expr . /etc/ncmm/env.sh && /entrypoint.sh $cmd > /proc/1/fd/1 2>&1" >> /etc/crontabs/root
      echo "Added cron job: $cron_expr -> ncmm $cmd"
    fi
  done
fi

# 方案B：从CRON_xxx环境变量解析定时任务
env | grep -E '^CRON' | while read -r env_var; do
  value=$(echo "$env_var" | cut -d'=' -f2-)
  [ -z "$value" ] && continue
  read -r m h dom mon dow cmd <<EOF
$value
EOF
  if [ -n "$m" ] && [ -n "$h" ] && [ -n "$dom" ] && [ -n "$mon" ] && [ -n "$dow" ] && [ -n "$cmd" ]; then
    cron_expr="$m $h $dom $mon $dow"
    if ! grep -qF "$cron_expr . /etc/ncmm/env.sh && /entrypoint.sh $cmd" /etc/crontabs/root; then
      echo "$cron_expr . /etc/ncmm/env.sh && /entrypoint.sh $cmd > /proc/1/fd/1 2>&1" >> /etc/crontabs/root
      echo "Added cron job (env): $cron_expr -> ncmm $cmd"
    fi
  else
    if [ -n "$m" ] && [ -n "$h" ] && [ -n "$dom" ] && [ -n "$mon" ] && [ -n "$dow" ]; then
      cron_expr="$m $h $dom $mon $dow"
      if ! grep -qF "$cron_expr . /etc/ncmm/env.sh && /entrypoint.sh task" /etc/crontabs/root; then
        echo "$cron_expr . /etc/ncmm/env.sh && /entrypoint.sh task > /proc/1/fd/1 2>&1" >> /etc/crontabs/root
        echo "Added cron job (env): $cron_expr -> ncmm task"
      fi
    fi
  fi
done

# ===== 【核心修改点2】同步完Cookie再启动crond，逻辑不变 =====
if [ -s /etc/crontabs/root ]; then
  echo "Starting crond daemon..."
  chmod 600 /etc/crontabs/root
  exec crond -f -l 8
fi

# 无定时任务时的单次执行逻辑（保留）
if [ "$1" = "cron" ]; then
  echo "Error: Cron mode requested but no valid schedules found."
  exit 1
fi

# 兜底执行（保留）
exec ncmm -c /data/config.yaml "$@"
