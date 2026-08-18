@echo off
setlocal
cd /d "%~dp0"
set "NCMM_EXE=%~dp0ncmm.exe"

if not exist "%NCMM_EXE%" (
    echo [ERROR] ncmm.exe was not found in "%~dp0".
    pause
    exit /b 1
)

"%NCMM_EXE%" web --scheduler --background
set "exit_code=%errorlevel%"

if not "%exit_code%"=="0" (
    echo.
    echo [ERROR] Failed to start ncmm. Exit code: %exit_code%.
    pause
    exit /b %exit_code%
)

start "" "http://127.0.0.1:3899"
exit /b 0
