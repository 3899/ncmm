# 插件机制与开发标准 (Plugin Specification)

`ncmm` 支持跨语言、跨平台的外部进程级插件系统（类似 Git / Kubectl 插件架构）。开发者可以使用 Go、Python、Shell 或任何编程语言编写独立扩展插件，无需修改 `ncmm` 主体源码即可无缝接入任务调度与通知体系。

---

## 一、 插件发现与目录规范

`ncmm` 启动或执行插件命令时，会按以下优先级自动扫描插件可执行文件：

1. **当前工作目录**：`./plugins/`（Docker 容器映射推荐 `/data/plugins/`）
2. **用户数据主目录**：`{home}/plugins/`（默认为 `~/.ncmm/plugins/` 或由 `--home` 指定）
3. **可执行文件同级目录**：`{ncmm_exe_dir}/plugins/`

### 1. 结构与命名支持
插件支持以下两种存放形态：

- **单文件形态（极简推荐）**：
  - Linux/macOS：`plugins/ncmm-<name>` 或 `plugins/<name>`（需 `chmod +x`）
  - Windows：`plugins/ncmm-<name>.exe`、`plugins/<name>.exe`、`plugins/<name>.py`、`plugins/<name>.bat`
- **子目录形态（适合带配置或资源的复杂插件）**：
  - `plugins/ncmm-<name>/ncmm-<name>`（或 `.exe`）
  - 例如：`plugins/ncmm-report/ncmm-report` 或 `plugins/ncmm-report/ncmm-report.exe`

### 2. 插件名称提取规则
程序会自动剥离前缀 `ncmm-`、`ncmm_` 以及系统后缀（如 `.exe`、`.py`、`.bat`）。
- 文件名 `ncmm-report.exe` -> 插件注册名称为 **`report`**
- 文件名 `ncmm_sync` -> 插件注册名称为 **`sync`**
- 调用命令为：`ncmm plugin run <name>`

---

## 二、 运行契约与输入规范 (Input Contract)

当 `ncmm` 调用插件时，会通过 **命令行参数透传**、**环境变量** 与 **标准输入 (stdin)** 传递宿主运行上下文。

### 1. 命令行参数透传 (CLI Arguments)
`ncmm plugin run` 会将跟随在插件名之后的所有参数原样透传给底层插件，**支持与插件独立运行完全一致的参数语法**：
```bash
# 方式 1：直接追加参数
ncmm plugin run report --days 7 --format json

# 方式 2：使用 POSIX 标准 -- 分隔符追加参数
ncmm plugin run report -- --days 7 --format json
```

### 2. 环境变量传递
宿主调用插件时会注入以下环境变量：

| 变量名 | 说明 | 示例 |
| :--- | :--- | :--- |
| `NCMM_PLUGIN` | 标记当前由 ncmm 宿主调用（值固定为 `"1"`） | `"1"` |
| `NCMM_HOME` | ncmm 数据与工作目录绝对路径 | `"/data"` 或 `"C:\Users\...\.ncmm"` |
| `NCMM_CONFIG_PATH` | 当前使用的 `config.yaml` 绝对路径 | `"/data/config.yaml"` |
| `NCMM_NOTIFY_FILE` | 当前通知配置文件 `notify.yaml` 路径 | `"/data/notify.yaml"` |
| `NCMM_ACCOUNT_COOKIE`| 当前待执行账号的 Cookie 文件路径 | `"/data/cookie.json"` |
| `NCMM_ACCOUNT_IS_MAIN`| 当前账号是否为主账号 | `"true"` 或 `"false"` |

### 3. 标准输入上下文 (Stdin JSON)
宿主启动插件进程时，还会向插件的 `stdin` 写入完整的 JSON 上下文（`PluginContext`）：
```json
{
  "version": "1.0.0",
  "command": "plugin run",
  "home": "/data",
  "config_path": "/data/config.yaml",
  "notify_file": "/data/notify.yaml",
  "account": {
    "filepath": "/data/cookie.json",
    "is_main": true
  }
}
```

---

## 三、 输出契约与通知规范 (Output Contract)

插件执行期间的所有普通日志可自由输出至 `stdout` 或 `stderr`，宿主会将其实时重定向输出到终端供用户查看。

在执行结束前，插件可在 `stdout` 最终输出一行标准的 **JSON 结果对象**（`PluginResult`）：

### 1. 结果 JSON 结构
```json
{
  "success": true,
  "message": "账号 cookie.json 数据报告生成成功",
  "data": {
    "total_records": 128,
    "status": "completed"
  },
  "notify": {
    "should_notify": true,
    "title": "网易云音乐 - 插件运行完成",
    "content": "【插件通知】数据报告已生成，详情请查看日志。",
    "level": "info"
  }
}
```

### 2. 字段详细说明
- **`success`** (bool, 必须)：任务是否成功。若为 `false`，宿主会提示错误。
- **`message`** (string, 必须)：执行摘要文本。
- **`data`** (object, 可选)：业务回传结构数据。
- **`notify`** (object, 可选)：需要宿主统一代理推送的消息对象：
  - `should_notify`: 设为 `true` 触发推送；
  - `title`: 通知标题；
  - `content`: 通知详细内容（支持换行与 Markdown）；
  - `level`: 消息级别，可选 `"info"`、`"warn"`、`"error"`。

---

## 四、 插件管理与使用方式

### 1. 插件列表与运行
```bash
# 查看所有已识别插件
ncmm plugin list

# 运行插件（默认参数）
ncmm plugin run report

# 运行插件并传递自定义参数
ncmm plugin run report --days 30 --export /data/export.csv

# 指定特定账号运行
ncmm plugin run report -a /path/to/cookie.json
```

### 2. 远程安装与私有仓库安装 (`ncmm plugin install`)
支持通过远程直链或 GitHub Release 远程一键安装插件：
```bash
# 1. 通过压缩包直链安装 (自动解压可执行文件至 plugins/ 目录并赋权)
ncmm plugin install https://example.com/ncmm-report_Linux_x86_64.tar.gz

# 2. 从 GitHub 公开或私有仓库远程安装 (配合 Token)
ncmm plugin install <用户名>/<仓库名> --token ghp_xxxxxxxxxxxx
```

### 3. WebUI 与调度说明
- **独立任务设计**：插件与日常打卡任务（`ncmm task`）保持独立解耦，日常打卡不隐式触发插件，确保打卡任务纯粹可控。
- **WebUI 调度**：可在 WebUI 的「定时任务」中新建独立的插件调度规则：
  - **执行命令**：`plugin`
  - **参数列表**：`run report --days 7`
  - **Cron 表达式**：`0 8 * * *`（例如每天早上 8:00 执行）

---

## 五、 快速上手：最小插件示例 (Python)

创建脚本 `plugins/ncmm-hello.py`：

```python
#!/usr/bin/env python3
import os
import sys
import json

def main():
    account_cookie = os.environ.get("NCMM_ACCOUNT_COOKIE", "cookie.json")
    print(f"[Hello-Plugin] 收到参数: {sys.argv[1:]}")
    
    # 构造标准返回结果
    result = {
        "success": True,
        "message": f"Hello 插件执行成功，账号: {account_cookie}",
        "notify": {
            "should_notify": True,
            "title": "Hello 插件通知",
            "content": f"参数: {sys.argv[1:]}",
            "level": "info"
        }
    }
    print(json.dumps(result))

if __name__ == "__main__":
    main()
```
赋予执行权限后（`chmod +x plugins/ncmm-hello.py`），执行 `ncmm plugin run hello --param1 value1` 即可体验！
