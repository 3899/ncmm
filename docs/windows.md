# Windows 运行指南

## 准备文件

从 Release 下载与系统架构对应的 Windows ZIP，并完整解压到单独目录。不要直接在压缩包中运行程序。

解压后的目录应至少包含：

```text
ncmm.exe
config.yaml
notify.yaml
一键启动.bat
一键停止.bat
```

## 前台运行

需要查看实时启动输出或排查错误时，建议在 PowerShell 中运行：

```powershell
cd C:\path\to\ncmm
.\ncmm.exe web --scheduler
```

CMD 中的等效命令：

```bat
cd /d C:\path\to\ncmm
ncmm.exe web --scheduler
```

启动后访问 [http://127.0.0.1:3899](http://127.0.0.1:3899)。首次启动且尚未配置管理员密码时，页面直接显示“设置管理员密码”，设置并确认后即可进入。密码只以 PBKDF2 加盐 hash 写入 `webui-auth.json`，浏览器通过 HttpOnly Session Cookie 登录。

需要允许局域网中的其他设备访问时，可以监听所有网卡，并在 Windows 防火墙中放行 TCP 端口 `3899`：

```powershell
.\ncmm.exe web --listen 0.0.0.0:3899 --scheduler
```

首次设置没有额外设置码。请先在受信任网络完成密码设置，再向不可信网络开放端口；公网访问应使用 HTTPS 反向代理并增加 `--secure-cookie`。

## 无窗口启动

双击 `一键启动.bat` 即可在后台运行：

```text
ncmm.exe web --scheduler --background
```

`一键启动.bat` 不依赖 PowerShell。`ncmm.exe` 会创建无控制台窗口的后台进程，启动完成后不会保留 CMD 窗口或任务栏按钮。

脚本会等待 WebUI 启动成功，然后自动使用默认浏览器打开 [http://127.0.0.1:3899](http://127.0.0.1:3899)。WebUI 默认只监听 `127.0.0.1:3899`。

首次启动且目录中的 `webui-auth.json` 尚未配置管理员密码时，浏览器会显示首次设置页面；后续启动显示密码登录页面。

v1.2.0 不兼容或迁移 v1.1.x 的 WebUI 管理令牌。若升级后没有 `webui-auth.json`，直接重新设置管理员密码；原有 `config.yaml`、Cookie、数据库、调度规则和运行记录都会保留。

忘记密码时先停止 WebUI，再在 PowerShell 中交互重置；输入不会回显，重置会撤销所有旧会话：

```powershell
.\ncmm.exe --home . auth reset-password
```

需要彻底回到首次设置状态时执行 `.\ncmm.exe --home . auth clear --yes`。该命令只清除 `webui-auth.json`，不会读取或删除旧认证文件，也不会修改任务数据。认证恢复不读取 `config.yaml`，因此主配置损坏时仍可使用。

## 停止运行

双击 `一键停止.bat`，或在 PowerShell/CMD 中执行：

```bat
taskkill /F /T /IM ncmm.exe
```

停止脚本会结束当前 Windows 用户能够访问的所有 `ncmm.exe` 进程。若同时运行了多个 ncmm 实例，它们会一起停止。
