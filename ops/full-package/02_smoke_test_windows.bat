@echo off
setlocal
cd /d "%~dp0"

echo Running Archive Center 2.1 Windows live smoke...
powershell -NoProfile -ExecutionPolicy Bypass -File ".\scripts\smoke-live.ps1"
pause
