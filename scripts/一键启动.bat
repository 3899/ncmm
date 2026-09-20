@echo off
setlocal
cd /d "%~dp0"
set "NCMM_EXE=%~dp0ncmm.exe"
set "NCMM_WEB_PRESERVE_LEGACY_SCHEDULES=true"

if not exist "%NCMM_EXE%" (
    echo [ERROR] ncmm.exe was not found in "%~dp0".
    pause
    exit /b 1
)

:: 启动服务（新窗口运行，避免阻塞批处理）
start "NCMM Server" "%NCMM_EXE%" web

:: 等待 2 秒，确保 Web 端口监听已就绪
timeout /t 2 /nobreak >nul

:: 打开浏览器访问
start "" "http://127.0.0.1:3899"
exit /b 0
