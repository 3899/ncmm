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

启动后访问 [http://127.0.0.1:3899](http://127.0.0.1:3899)。首次启动且未指定管理令牌时，页面会要求设置并确认至少 8 位的管理令牌；设置成功后写入运行目录下的 `webui.secret`。

可以在命令行指定管理令牌：

```powershell
.\ncmm.exe web --scheduler --token "your-token"
```

需要允许局域网中的其他设备访问时，可以监听所有网卡。此时应设置强管理令牌，并在 Windows 防火墙中放行 TCP 端口 `3899`：

```powershell
.\ncmm.exe web --listen 0.0.0.0:3899 --scheduler --token "your-strong-token"
```

## 无窗口启动

双击 `一键启动.bat` 即可在后台运行：

```text
ncmm.exe web --scheduler --background
```

`一键启动.bat` 不依赖 PowerShell。`ncmm.exe` 会创建无控制台窗口的后台进程，启动完成后不会保留 CMD 窗口或任务栏按钮。

脚本会等待 WebUI 启动成功，然后自动使用默认浏览器打开 [http://127.0.0.1:3899](http://127.0.0.1:3899)。WebUI 默认只监听 `127.0.0.1:3899`。

首次启动且目录中不存在 `webui.secret` 时，浏览器会显示首次设置页面，由用户自行设置登录令牌。后续启动会显示正常登录页面，不再进入首次设置流程。

## 停止运行

双击 `一键停止.bat`，或在 PowerShell/CMD 中执行：

```bat
taskkill /F /T /IM ncmm.exe
```

停止脚本会结束当前 Windows 用户能够访问的所有 `ncmm.exe` 进程。若同时运行了多个 ncmm 实例，它们会一起停止。
