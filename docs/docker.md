# Docker 部署指南

预览图：

![webui](.\pic\webui.jpg)



Docker 镜像默认启动 WebUI 和内置定时任务调度器，访问地址为 `http://localhost:3899`。运行数据、配置、Cookie 和日志统一保存在挂载的 `/data` 目录。

镜像启动命令是 `ncmm web --listen 0.0.0.0:3899`；`web` 本身已经包含调度能力，不需要额外参数。每条定时规则通过自身开关独立启停，并按各自 cron 触发。



## Docker Compose

```yaml
services:
  ncmm:
    image: ghcr.io/3899/ncmm:latest
    container_name: ncmm
    restart: unless-stopped
    ports:
      - "127.0.0.1:3899:3899"
    volumes:
      - ./data:/data
    extra_hosts:
      - "host.docker.internal:host-gateway"
    environment:
      - TZ=Asia/Shanghai
      # HTTPS 反向代理访问时启用 Secure Session Cookie。
      # - NCMM_WEB_SECURE_COOKIE=true
      # - COOKIECLOUD_SERVER=http://host.docker.internal:8088
      # - COOKIECLOUD_UUID=your-uuid
      # - COOKIECLOUD_PASSWORD=your-password
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
- `webui-auth.json`：PBKDF2 管理员密码 hash、Session hash、策略和会话元数据，不含明文密码或明文 Session Token。
- `webui.instance.lock`：WebUI 单实例锁及诊断元数据；容器停止后文件可保留，不能仅据此判断服务是否运行。
- `log/runs/`：定时任务运行日志。

## WebUI

打开 `http://localhost:3899`。`/data/webui-auth.json` 尚未配置管理员密码时，页面直接显示与 SimAdmin 一致的首次设置界面，只需设置并确认管理员密码。密码只保存 PBKDF2 加盐 hash；登录成功后浏览器使用 HttpOnly、SameSite=Strict Session Cookie，服务端只保存 Session Token hash。

Compose 默认使用 `127.0.0.1:3899:3899`，只从宿主机本机发布端口。容器内部因网络转发监听 `0.0.0.0`，但首次设置不再要求额外设置码；需要通过局域网或公网访问时，应先在宿主机完成首次设置，再开放端口或 HTTPS 反向代理，并设置 `NCMM_WEB_SECURE_COOKIE=true`。

v1.2.0 不迁移旧 WebUI 令牌。升级后若 `/data/webui-auth.json` 不存在，重新设置管理员密码即可；挂载目录中的 `config.yaml`、Cookie、数据库、`webui.yaml` 调度规则与 `log/runs` 运行记录不会被清理或重建。

旧 v1 `webui.yaml` 会迁移到 schema v2。官方镜像通过内部一次性迁移提示保留旧任务的启用状态；迁移完成后该提示不再影响 scheduler，后续只由每条任务自身的启用状态控制。

同一个 `/data` 卷只能由一个 WebUI 容器实例托管。即使映射到不同宿主机端口，第二个共享该数据卷的实例也会被 home 级实例锁拒绝，避免两套调度器重复执行任务。横向运行多个实例时必须分别挂载独立数据目录。

WebUI 支持：

- 带字段说明可视化编辑 `config.yaml` 和 `notify.yaml`，也可直接编辑 YAML。
- 支持粘贴 Cookie 或二维码登录，并可自定义导出的 `.json` 文件名。
- 创建、修改、启停和立即运行定时任务。
- 查看、停止运行任务以及查看运行日志。
- 按保留天数和最大容量自动清理日志。
- 修改管理员密码与密码/会话策略，查看和撤销登录会话，以及手动检查新版本。

忘记管理员密码时，先停止容器，再用相同 `/data` 卷运行一次恢复命令；密码通过标准输入传入，不出现在进程参数中：

```bash
printf '%s\n' 'New#Password123' | docker run --rm -i -v "$(pwd)/data:/data" ghcr.io/3899/ncmm:latest --home /data auth reset-password --password-stdin
```

### 任务并发

内置调度器默认最多运行 1 个任务，无需额外配置。可在“运行记录”的运行设置中将最大并发调整为 1–8；即使调高，不同规则使用同一账号或共享数据库时仍会自动排队。

规则的“跳过重复触发”只针对同一规则已有 running/queued 记录的场景，并会生成可查询的 `skipped` 运行记录。“重复触发进入队列”允许保存本次触发，但账号和数据库资源可用前不会启动子进程。排队记录可以和运行中记录一样手动停止。

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

不要在同一个容器中同时使用默认 Web 管理模式和下方 BusyBox `crond` 模式；两者是互斥的调度入口，否则可能重复执行相同业务。

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
  -p 127.0.0.1:3899:3899 \
  -v ./data:/data \
  -e TZ=Asia/Shanghai \
  ghcr.io/3899/ncmm:latest
```
