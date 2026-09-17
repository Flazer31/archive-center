@echo off
setlocal
cd /d "%~dp0"
powershell -NoProfile -ExecutionPolicy Bypass -File ".\scripts\export-diagnostics.ps1"
set "AC_DIAGNOSTIC_EXIT=%ERRORLEVEL%"
pause
exit /b %AC_DIAGNOSTIC_EXIT%
