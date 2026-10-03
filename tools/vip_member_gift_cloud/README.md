# VIP Member Gift Cloud 服务

这是一个为网易云音乐 VIP 赠送（黑胶会员分享）提供支持的单文件 Python 云端服务。
它仅使用 Python 标准库 `http.server` + `sqlite3` 实现，无须安装 FastAPI、Uvicorn 等任何第三方依赖即可运行。

## 接口说明

服务已包含并实现了以下核心接口：
* `POST /tokens/upsert`：发布/刷新赠送 token
* `GET /tokens/available`：给领取端取可用 token（支持 `?priorityDonors=UID1,UID2` 动态传参指定优先赠送者列表）
* `GET /claims/status`：查询当前账号本月是否已领
* `POST /claims/success`：领取成功后按 `accept.duration` 扣除可用天数
* `POST /tokens/fail`：标记 token 对某账号失败，或全局过期/耗尽。`reason` 取值：
  * `invalid` / `expired` / `depleted` / `exhausted` / `unavailable` 为**终态**，会把 token 全局标记为该状态，不再发放给任何人；
  * 其它值（含默认 `failed`）为**非终态**，只把该接收者短期拉黑（见 `VIP_GIFT_FAILURE_EXPIRE_MS`），token 对其他人依然可用；
  * 为避免坏卡被反复发放，服务端会把 `message` 中含“不是会员”“12200”“12206”“非法”的失败统一升级为终态 `invalid`。
* `GET/POST /claims/failures/clear`：清空记录的防重复获取失败关联。支持 `?receiverUid=xxx` 清理单账号，或不传清理全局。
* `POST /maintenance/prune`：删除可用天数小于 7、月份过期或时间过期的 token
* `GET /health`：服务健康检查（包含当前已生效的 `priorityDonors` 优先级配置）
* `GET /stats`：运行数据与 token 统计

## 部署方法

支持本地直接运行，或者使用 Docker 部署。服务默认运行在 `3102` 端口。
Docker 镜像已基于轻量级 `python:3.12-alpine` 构建，解压后体积仅约 **45MB**（压缩下载仅约 **18MB**），内存占用极低。

### 方法一：本地部署

1. **环境要求**：Python 3.8 或更高版本。
2. **运行服务**：
   在终端进入当前目录，直接执行该脚本：
   ```bash
   python vip_member_gift_cloud.py
   ```
3. **参数配置（可选）**：
   可以通过环境变量或命令行参数来配置服务行为：
   * `VIP_GIFT_DB_PATH` 或 `--db`：SQLite 数据库存放路径，默认值为 `vip_member_gift_cloud.sqlite3`
   * `VIP_GIFT_CLOUD_TOKEN` 或 `--token`：开启 API 认证。设置后，除 `/health` 外其他接口请求 Header 中必须携带 `Authorization: Bearer <token>` 或 `X-Api-Key: <token>`
   * `VIP_GIFT_HOST` 或 `--host`：服务绑定的主机地址，默认为 `0.0.0.0`
   * `VIP_GIFT_PORT` 或 `--port`：服务绑定的端口，默认为 `3102`
   * `VIP_GIFT_MIN_AVAILABLE_DAYS` 或 `--min-available-days`：token 的最少所需可用天数（低于该天数则不可再分发），默认 `7`
   * `VIP_GIFT_PRIORITY_DONORS` 或 `--priority-donors`：优先领取的赠送者 UID 列表（支持逗号分隔，例如 `"123456,87654321"`）。**按指定先后顺序优先分配**，只有在指定人的赠送码满足可用条件（天数充足、未过期、未被记录失败且非自领）时才优先返回；若指定人的码不可用或领完，**自动平滑回退到普通按剩余可用天数降序匹配**。
   * `VIP_GIFT_FAILURE_EXPIRE_MS` (环境变量)：由于用户配置错误或网络原因造成的领取失败，被记录排除时的过期时间（毫秒）。默认值：`3600000`（1小时）。超时后会允许重新请求，也可调用 `/claims/failures/clear` 自助清空。

   示例：配置指定人优先与自定义 Token 运行
   ```bash
   VIP_GIFT_CLOUD_TOKEN="your_secure_token" VIP_GIFT_PRIORITY_DONORS="123456,87654321" python vip_member_gift_cloud.py --port 8080
   ```

### 方法二：Docker 部署

目录中已包含 `Dockerfile` 和 `docker-compose.yml`。推荐使用 Docker Compose 进行管理。

1. **构建镜像**：
   如果你是第一次使用，先构建 Docker 镜像（基于极轻量 `python:3.12-alpine`）：
   ```bash
   docker build -t ncmm-vip-member-gift-cloud .
   ```

2. **启动服务**：
   在当前目录运行以下命令后台启动服务：
   ```bash
   docker compose up -d
   ```

3. **配置参数**：
   编辑同目录下的 `docker-compose.yml`，在 `environment` 节点下配置你的密钥与优先级名单，例如：
   ```yaml
   environment:
     VIP_GIFT_CLOUD_TOKEN: "你的强密码"
     # 优先领取的赠送者 UID 列表，逗号分隔，按先后顺序优先（可选）
     VIP_GIFT_PRIORITY_DONORS: "123456,87654321"
   ```
   **注意**：请务必将预设的 `xxxxxxxxx` 修改为你自己的 Token，以防止他人恶意调用你的接口。

4. **数据持久化**：
   `docker-compose.yml` 中已经自动将容器内的 `/data` 目录映射到本地的 `./vip-gift-data` 文件夹下。产生的 SQLite 数据库文件会自动保存在该目录中，即使重启或重建容器也不会丢失数据。

5. **常用命令**：
   * 查看运行日志：`docker compose logs -f`
   * 停止并删除容器：`docker compose down`
