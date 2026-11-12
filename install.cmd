@echo off
setlocal
cd /d "%~dp0"
powershell.exe -NoProfile -ExecutionPolicy RemoteSigned -File "%~dp0install.ps1"
if errorlevel 1 (
  echo.
  echo Setup stopped. See docs\installer-hosts.md for Windows security or runtime setup.
  pause
  exit /b 1
)
