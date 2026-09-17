@echo off
setlocal
cd /d "%~dp0"
if not exist ".env.full.local" if not exist ".env.full.local.protected" (
  copy ".env.full.example" ".env.full.local" >nul
)
powershell -NoProfile -ExecutionPolicy Bypass -File ".\scripts\start-full-windows.ps1" -ConfigurePorts %*
set "ARCHIVE_CENTER_EXIT_CODE=%ERRORLEVEL%"
pause
exit /b %ARCHIVE_CENTER_EXIT_CODE%
