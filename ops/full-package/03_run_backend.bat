@echo off
setlocal
cd /d "%~dp0"

echo Archive Center 2.1 uses one Windows full-package launcher.
echo Starting through 01_start_archive_center_windows.bat...
echo.
call "%~dp001_start_archive_center_windows.bat"

endlocal
