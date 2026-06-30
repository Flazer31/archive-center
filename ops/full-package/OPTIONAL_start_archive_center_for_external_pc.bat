@echo off
setlocal
cd /d "%~dp0"

if not exist ".env.full.local" (
  echo [Archive Center 2.1] Creating .env.full.local from .env.full.example
  copy ".env.full.example" ".env.full.local" >nul
)

echo.
echo ============================================================
echo  Archive Center 2.1 External PC Mode
echo ============================================================
echo.
echo  This compatibility launcher now delegates to:
echo    01_start_archive_center_windows.bat
echo.
echo  Use this only when another PC must connect to this backend.
echo  You may need to allow TCP 28080 in Windows Firewall,
echo  Tailscale, or your proxy setup.
echo.
echo  For same-PC use, close this window and run:
echo    01_start_archive_center_windows.bat
echo.
echo ============================================================
echo.

call "%~dp001_start_archive_center_windows.bat"

endlocal
