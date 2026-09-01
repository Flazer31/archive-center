@echo off
setlocal
cd /d "%~dp0"

if not exist ".env.full.local" if not exist ".env.full.local.protected" (
  echo [Archive Center 2.1] Creating .env.full.local from .env.full.example
  copy ".env.full.example" ".env.full.local" >nul
)

echo.
echo ============================================================
echo  Archive Center 2.1 Windows Package
echo ============================================================
echo.
echo  Server will start now.
echo  Keep this window open while using Archive Center.
echo.
echo  RisuAI plugin file:
echo    Archive Center.js
echo.
echo  Backend URL:
echo    Same PC:       http://127.0.0.1:28080
echo    Remote device: http://SERVER_IP_OR_DOMAIN:28080
echo.
echo  The backend listens on 0.0.0.0:28080 so the same launcher
echo  works for both local and remote RisuAI browsers.
echo  To stop, press Ctrl+C. At the Archive Center prompt choose Y.
echo  Choose N there to keep every managed service running.
echo.
echo ============================================================
echo.
powershell -NoProfile -ExecutionPolicy Bypass -File ".\scripts\start-full-windows.ps1" -RuntimeProfile "full_local" -VectorMode "bundled" -BindAddr "0.0.0.0:28080"
set "ARCHIVE_CENTER_EXIT_CODE=%ERRORLEVEL%"
if not "%ARCHIVE_CENTER_EXIT_CODE%"=="0" (
  echo.
  echo Archive Center launcher stopped with exit code %ARCHIVE_CENTER_EXIT_CODE%.
  pause
)
exit /b %ARCHIVE_CENTER_EXIT_CODE%
