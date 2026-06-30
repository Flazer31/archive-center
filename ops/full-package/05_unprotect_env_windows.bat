@echo off
setlocal
cd /d "%~dp0"

echo.
echo ============================================================
echo  Unprotect Archive Center env
echo ============================================================
echo.
echo  This restores .env.full.local from:
echo    .env.full.local.protected
echo.
echo  Use this only when you need to edit local settings.
echo  Run 04_protect_env_windows.bat again after editing.
echo.
echo ============================================================
echo.
powershell -NoProfile -ExecutionPolicy Bypass -File ".\scripts\unprotect-env-windows.ps1" -ProtectedEnvFile ".\.env.full.local.protected" -OutputEnvFile ".\.env.full.local" -Force
pause

endlocal
