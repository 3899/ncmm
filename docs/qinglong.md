# NCMM 青龙 / 呆呆面板使用指南

本项目提供在青龙面板（Qinglong）与呆呆面板（Dumb-Panel / daidai）环境下一键部署、账号登录导入及自动化任务执行的 Python 脚本。

标准面板部署使用青龙自身的 cron 定期执行 `./ncmm task`，不会启动 WebUI、监听 `3899` 或加载内置 Go 调度器。不要把 `ncmm web` 配置成普通青龙定时任务，也不要同时用面板 cron 和 WebUI 定时规则执行同一组任务。

---

## 📌 订阅仓库配置

在青龙面板的 **订阅管理** 中添加订阅，或在终端直接运行以下指令：

```bash
ql repo https://github.com/3899/ncmm.git "ncmm-" "" "" "" "py"
```

> 💡 **国内镜像备用**：若 GitHub 连接较慢，可使用代理地址：
> `ql repo https://gh-proxy.com/https://github.com/3899/ncmm.git "ncmm-" "" "" "" "py"`

---

## 🚀 首次部署与使用步骤

### Step 1. 安装/更新二进制主程序
1. 在青龙面板的定时任务列表中找到 **`NCMM 安装、更新`** (`ncmm-update.py`)。
2. 手动运行该脚本，系统会自动识别宿主机/容器平台与架构，并下载最新的 `ncmm` 二进制文件与默认配置文件。

---

### Step 2. 配置账号登录方式（推荐青龙环境变量）

`ncmm-login.py` 智能支持以下三种登录配置方式（按优先级自动检测导入）：

#### 方式 1：CookieCloud 自动同步（最方便）
若使用 CookieCloud 服务，在青龙面板 **环境变量** 页面添加：

| 变量名 | 说明 | 示例 |
| :--- | :--- | :--- |
| **`NCMM_COOKIECLOUD_UUID`** | CookieCloud 用户 UUID | `a1b2c3d4-xxxx-xxxx` |
| **`NCMM_COOKIECLOUD_PASSWORD`** | CookieCloud 解密密码 | `mysecretpass` |
| **`NCMM_COOKIECLOUD_SERVER`** | CookieCloud 服务端地址（可选） | `http://127.0.0.1:8088`（默认） |

#### 方式 2：青龙环境变量（推荐，零文件依赖，支持主/辅账号分离）
在青龙面板 **环境变量** 页面添加：

| 变量名 | 说明 | 示例 |
| :--- | :--- | :--- |
| **`NCMM_MAIN_COOKIE`** | **主账号** Cookie 或 `MUSIC_U` Token | `MUSIC_U=xxx` |
| **`NCMM_SECONDARY_COOKIE`** | **辅助账号** (`secondary`) Cookie 或 Token<br>(多个账号可用 `&`、`@` 或换行分隔) | `MUSIC_U=yyy&MUSIC_U=zzz` |
| **`NCMM_COOKIE`** | **通用 Cookie 变量**（若未显式分主辅，首个自动为主账号，后续为辅助账号） | `MUSIC_U=xxx&MUSIC_U=yyy` |

#### 方式 3：本地文件显式导入（与方式 2 具有完全相同的逻辑结构）
按需配置指定的本地 Cookie 文件路径：

| 变量名 | 说明 | 默认/示例 |
| :--- | :--- | :--- |
| **`NCMM_MAIN_COOKIE_FILE`** | **主账号** Cookie 文件路径 | `main_cookie.txt` |
| **`NCMM_SECONDARY_COOKIE_FILE`** | **辅助账号** Cookie 文件路径（多个文件可用逗号 `,`、分号 `;` 或 `&` 分隔） | `fan1.txt,fan2.txt` |
| **`NCMM_COOKIE_FILE`** | **通用 Cookie 文件变量**（首个文件为主账号，后续为辅助账号） | 默认使用同级 `cookie.txt` |

#### 方式 4：自动更新与版本检测控制（可选）
通过环境变量灵活控制主程序的更新检测与热替换行为：

| 变量名 | 说明 | 默认/示例 |
| :--- | :--- | :--- |
| **`NCMM_UPDATER_CHECK`** | 是否开启版本检测（填 `false` 或 `0` 禁用） | `true` |
| **`NCMM_UPDATER_AUTO_UPDATE`** | 是否开启二进制自动热更新（填 `false` 或 `0` 禁用） | `true` |

---

### Step 3. 导入账号生成 Cookie 配置文件
1. 在青龙面板中手动运行 **`NCMM 账号登录`** (`ncmm-login.py`) 脚本。
2. 脚本会自动按优先级检测 **CookieCloud -> 环境变量 -> 本地文件**，校验通过后自动生成/更新 **`cookie.json`** 及配置文件。
3. 如果需要自定义 `config.yaml` 配置文件，请在此步完成后根据需求修改同目录下的 `config.yaml`（例如调整任务开关、推歌/打卡参数等）。

---

### Step 4. 运行日常自动化任务
1. 在青龙面板中找到 **`NCMM 任务执行`** (`ncmm-run.py`) 脚本。
2. 手动运行一次测试任务是否正常执行。
3. 该任务默认已配置每日定时触发（`9 0,13 * * *`），后续青龙面板将每日自动为您执行签到与打卡任务。

---

## 💡 使用注意事项

* **定时任务禁用**：`ncmm-update.py`（NCMM 安装、更新）和 `ncmm-login.py`（NCMM 账号登录导入）仅在**首次部署、导入新账号或后续更新版本**时需要运行。首次配置完成后，**可以在青龙面板中直接禁用这两个任务**，防止日常误触发；后续需要更新或重新登录时再开启手动运行即可。

---

## 📝 脚本说明速查

| 脚本文件名 | 青龙任务名称 | 推荐 Cron | 说明 |
| :--- | :--- | :--- | :--- |
| **`ncmm-update.py`** | `NCMM 安装、更新` | `0 0 1 1 *` | 首次部署或手动更新程序时运行（运行后可禁用） |
| **`ncmm-login.py`** | `NCMM 账号登录` | `0 0 1 1 *` | 自动解析环境变量/CookieCloud/本地文件导入账号（运行后可禁用） |
| **`ncmm-run.py`** | `NCMM 任务执行` | `9 0,13 * * *` | 每日0点9分, 13点9分: 自动跑脚本（执行 `./ncmm task`） |
