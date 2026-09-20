@echo off
setlocal
cd /d "%~dp0"
set "NCMM_EXE=%~dp0ncmm.exe"

if not exist "%NCMM_EXE%" (
    echo [ERROR] ncmm.exe was not found in "%~dp0".
    pause
    exit /b 1
)

"%NCMM_EXE%" --home "%~dp0" web stop
if errorlevel 1 (
    echo [ERROR] Failed to stop the WebUI for "%~dp0".
    pause
    exit /b 1
)

timeout /t 2 /nobreak >NUL
