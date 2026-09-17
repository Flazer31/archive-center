@echo off
setlocal
cd /d "%~dp0"

if not exist ".env.full.local" (
  if exist ".env.full.local.protected" (
    echo [Archive Center 2.1] .env.full.local.protected already exists.
    echo The plaintext .env.full.local file is not present.
    pause
    endlocal
    exit /b 0
  )
  echo [Archive Center 2.1] Creating .env.full.local from .env.full.example
  copy ".env.full.example" ".env.full.local" >nul
)

echo.
echo ============================================================
echo  Protect Archive Center env
echo ============================================================
echo.
echo  This encrypts .env.full.local into:
echo    .env.full.local.protected
echo.
echo  Windows DPAPI ties it to the current Windows user account.
echo  After protection, the plaintext .env.full.local is removed.
echo.
echo ============================================================
echo.
powershell -NoProfile -ExecutionPolicy Bypass -File ".\scripts\protect-env-windows.ps1" -EnvFile ".\.env.full.local" -RemovePlaintext -Force
pause

endlocal
