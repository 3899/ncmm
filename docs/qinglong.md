# NCMM 青龙 / 呆呆面板使用指南

本项目提供在青龙面板（Qinglong）与呆呆面板（Dumb-Panel / daidai）环境下一键部署、账号登录导入及自动化任务执行的 Python 脚本。

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

### Step 2. 准备 Cookie 文本文件
1. 登录服务器/青龙面板，进入脚本所在的对应目录（通常为 `/ql/data/scripts/3899_ncmm_qinglong/` 或 `/ql/scripts/ncmm/`）。
2. 在该目录下新建 **`cookie.txt`** 文件。
3. 将抓包获取到的网易云账号 Cookie 字符串复制并粘贴保存至 `cookie.txt` 中。
> 💡 **小提示**：默认 Cookie 文件名为 `cookie.txt`。如需使用其他文件名（例如 `fan1.txt`），请参考 Step 3 修改 `ncmm-login.py`。

---

### Step 3. 导入账号生成 Cookie 配置文件
1. 若需使用自定义的 Cookie 文件名（例如 `fan1.txt`），请先编辑 `ncmm-login.py` 脚本顶部的 `COOKIE_TXT_FILE` 变量值（如 `COOKIE_TXT_FILE = "fan1.txt"`）。
2. 在青龙面板中手动运行 **`NCMM 账号登录导入`** (`ncmm-login.py`) 脚本。
3. 脚本会自动读取同目录下的 Cookie 文件，解析校验后自动在同级生成对应的 **`cookie.json`**（或与自定义文件名相对应的 `.json`）账号配置文件。
4. 如果需要自定义 `config.yaml` 配置文件，请在此步完成后根据需求修改同目录下的 `config.yaml`（例如调整任务开关、推歌/打卡参数等）。

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
| **`ncmm-login.py`** | `NCMM 账号登录导入` | `0 0 1 1 *` | 将 `cookie.txt` 转换导出为 `cookie.json`（运行后可禁用） |
| **`ncmm-run.py`** | `NCMM 任务执行` | `9 0,13 * * *` | 每日0点9分, 13点9分: 自动跑脚本（执行 `./ncmm task`） |
