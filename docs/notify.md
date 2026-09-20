# 失败通知

运行失败时（未登录、任务报错、可选的 skip）在进程结束时汇总推送 **一条** 消息。成功不推送。

## 配置拆分

| 文件 | 内容 |
|------|------|
| `config.yaml` → `notify` | 策略：开关、是否推 skip、标题前缀、超时、通道文件路径 |
| `notify.yaml` | 通道凭证（Webhook / Bark / TG / 钉钉等） |

示例通道文件见仓库`config`目录的 `notify.yaml`，复制为 `notify.yaml` 后填写。

### config.yaml

```yaml
notify:
  enabled: false       # 总开关，如需开启改为：true
  on_skip: false       # skip 是否纳入汇总，默认 false
  title_prefix: "ncmm" # 推送标题前缀，便于多机区分
  timeout: 10s
  file: "notify.yaml" # 相对路径：相对 config.yaml 所在目录
```

### 启用步骤

1. `config.yaml` 设置 `notify.enabled: true`
2. 复制 `notify.yaml`（与 config.yaml 同目录或 `notify.file` 指定路径），配置对应推送通道需要的参数
3. 打开需要的通道并填写凭证

## 支持通道

- 自定义 Webhook（`body_template` 支持 `{{title}}` `{{content}}` `{{level}}` `{{time}}` `{{host}}`）
- Bark
- Server酱 / Server酱 Turbo
- Telegram
- 钉钉机器人
- QQ CoolPush (Qmsg)
- PushPlus
- 企业微信群机器人
- 企业微信应用

## 环境变量（可选，覆盖/补齐密钥）

| 变量 | 说明 |
|------|------|
| `NCMM_NOTIFY_WEBHOOK_URL` | Webhook URL |
| `NCMM_BARK_KEY` | Bark key |
| `NCMM_BARK_SERVER` | Bark 自建地址 |
| `NCMM_SCKEY` | Server酱 |
| `NCMM_TG_BOT_TOKEN` / `NCMM_TG_USER_ID` | Telegram |
| `NCMM_TG_API_HOST` / `NCMM_TG_PROXY` | Telegram 反代/代理 |
| `NCMM_DD_BOT_ACCESS_TOKEN` / `NCMM_DD_BOT_SECRET` | 钉钉 |
| `NCMM_QQ_SKEY` / `NCMM_QQ_MODE` | CoolPush |
| `NCMM_PUSH_PLUS_TOKEN` | PushPlus |
| `NCMM_QYWX_KEY` | 企微群机器人 |
| `NCMM_QYWX_AM` | 企微应用：`corpid,secret,touser,agentid[,media_id]` |

设置对应 env 时会自动启用该通道（无需 yaml 中 enabled）。

## 行为说明

- `ncmm task` 与独立命令（`sign` / `playids` / …）均会收集，并在进程结束时汇总推送
- 通知发送失败只写日志，不影响任务结果与退出码
- 无失败且（`on_skip=false` 或无 skip）时不推送
