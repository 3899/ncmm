@echo off
setlocal
cd /d "%~dp0"

if not exist "ncmm.exe" (
    echo [ERROR] ncmm.exe was not found in "%~dp0".
    pause
    exit /b 1
)

ncmm.exe web --scheduler
set "exit_code=%errorlevel%"

if not "%exit_code%"=="0" (
    echo.
    echo [ERROR] ncmm exited with code %exit_code%.
    pause
)

exit /b %exit_code%
