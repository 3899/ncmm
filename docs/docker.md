# Docker 部署指南

预览图：

![webui](.\pic\webui.jpg)



Docker 镜像默认启动 WebUI 和内置定时任务调度器，访问地址为 `http://localhost:3899`。运行数据、配置、Cookie 和日志统一保存在挂载的 `/data` 目录。



## Docker Compose

```yaml
services:
  ncmm:
    image: ghcr.io/3899/ncmm:latest
    container_name: ncmm
    restart: unless-stopped
    ports:
      - "3899:3899"
    volumes:
      - ./data:/data
    extra_hosts:
      - "host.docker.internal:host-gateway"
    environment:
      - TZ=Asia/Shanghai
      # 推荐显式设置；留空会自动生成并保存至 /data/webui.secret。
      # - NCMM_WEB_TOKEN=replace-with-a-long-random-token
      #- COOKIECLOUD_SERVER=http://host.docker.internal:8088
      #- COOKIECLOUD_UUID=your-uuid
      #- COOKIECLOUD_PASSWORD=your-password
```

启动容器：

```bash
docker compose up -d
docker logs ncmm
```

首次启动会在 `./data` 生成：

- `config.yaml`：ncmm 任务配置。
- `notify.yaml`：通知通道配置。
- `webui.yaml`：WebUI 定时任务和日志保留设置。
- `webui.secret`：未设置 `NCMM_WEB_TOKEN` 时生成的管理令牌。
- `log/runs/`：定时任务运行日志。

## WebUI

打开 `http://localhost:3899`，输入 `NCMM_WEB_TOKEN`，或输入 `docker logs ncmm` 首次启动时显示的管理令牌。

WebUI 支持：

- 带字段说明可视化编辑 `config.yaml` 和 `notify.yaml`，也可直接编辑 YAML。
- 粘贴 Cookie 登录，自定义导出的 `.json` 文件名。
- 创建、修改、启停和立即运行定时任务。
- 查看、停止运行任务以及查看运行日志。
- 按保留天数和最大容量自动清理日志。
- 修改管理令牌，以及手动检查新版本。

## 更新镜像

官方 Docker 镜像会强制禁用容器内二进制自更新，即使 `config.yaml` 中的 `updater.auto_update` 为 `true` 也不会替换 `/usr/local/bin/ncmm`。这样可避免容器可写层与镜像版本不一致。

WebUI 检查到新版本后，在宿主机执行：

```bash
docker compose pull ncmm
docker compose up -d --force-recreate ncmm
```

项目根目录的 `docker-compose.yml` 同时保留了 `build: .` 供本地源码构建。上述 `pull` 会刷新 `image` 指向的官方镜像；如需使用当前源码，则执行 `docker compose build --pull ncmm && docker compose up -d ncmm`。

## CookieCloud

Compose 使用桥接网络，因此宿主机上的 CookieCloud 地址应填写 `http://host.docker.internal:8088`。如果 CookieCloud 是同一个 Compose 项目中的服务，请改用其服务名，例如 `http://cookiecloud:8088`。

## 旧版 Cron 兼容

旧版 `CRON_*` 环境变量仍然有效：

```yaml
environment:
  - CRON_1=30 8 * * * task
  - CRON_2=0 14 * * * musician
```

这些规则由 Go 调度器载入，并在 WebUI 中显示为“环境变量托管”的只读规则。需要可视化修改时，应删除对应环境变量并在 WebUI 中重新创建。

显式使用旧版 BusyBox `crond` 仍受支持：

```yaml
command:
  - cron
  - |
    30 8 * * * task
    0 14 * * * musician
```

该模式不会启动 WebUI。

## 单次运行

```bash
docker compose run --rm ncmm task
docker compose run --rm ncmm sign
docker compose run --rm ncmm playids --ids 3366663042
docker compose run --rm ncmm musician
docker compose run --rm ncmm --help
```

## docker run

```bash
docker run -d --name ncmm \
  --restart unless-stopped \
  -p 3899:3899 \
  -v ./data:/data \
  -e TZ=Asia/Shanghai \
  -e NCMM_WEB_TOKEN=replace-with-a-long-random-token \
  ghcr.io/3899/ncmm:latest
```
