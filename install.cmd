@echo off
setlocal
cd /d "%~dp0"
if exist ".venv\Scripts\python.exe" goto environment_ready
where py >nul 2>nul
if errorlevel 1 (
  python -m venv .venv
) else (
  py -3 -m venv .venv
)
if errorlevel 1 goto failed
:environment_ready
".venv\Scripts\python.exe" -c "import sys; assert sys.version_info >= (3, 12), 'Python 3.12+ required'; import tkinter"
if errorlevel 1 goto failed
".venv\Scripts\python.exe" -c "import paramiko" >nul 2>nul
if not errorlevel 1 goto launch
".venv\Scripts\python.exe" -m pip install -r requirements.txt
if errorlevel 1 goto failed
:launch
".venv\Scripts\python.exe" -m flasher
if errorlevel 1 goto failed
exit /b 0
:failed
echo Setup stopped. Install Python 3.12+ with Tcl/Tk, review the error above, and see docs\installation-handoff.md.
pause
exit /b 1
