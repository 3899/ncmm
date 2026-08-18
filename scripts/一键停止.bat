@echo off
setlocal

tasklist /FI "IMAGENAME eq ncmm.exe" 2>NUL | find /I "ncmm.exe" >NUL
if errorlevel 1 (
    echo ncmm is not running.
    timeout /t 2 /nobreak >NUL
    exit /b 0
)

taskkill /F /T /IM ncmm.exe
if errorlevel 1 (
    echo [ERROR] Failed to stop ncmm.
    pause
    exit /b 1
)

echo ncmm stopped.
timeout /t 2 /nobreak >NUL
