@echo off
setlocal

set "SCRIPT_DIR=%~dp0"
set "PS_SCRIPT=%SCRIPT_DIR%start-dev.ps1"

if not exist "%PS_SCRIPT%" (
  echo start-dev.ps1 bulunamadi: "%PS_SCRIPT%"
  pause
  exit /b 1
)

powershell -NoProfile -ExecutionPolicy Bypass -File "%PS_SCRIPT%"
if errorlevel 1 (
  echo.
  echo Servisler baslatilirken hata olustu.
  pause
  exit /b 1
)

endlocal
