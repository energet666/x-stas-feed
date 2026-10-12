@echo off
rem Starts Feed AI from the package folder and opens it in the default browser.
cd /d "%~dp0"

start "" /min cmd /c "timeout /t 2 /nobreak >nul & start http://localhost:8080"

server.exe -addr :8080 -content-dir test-content -static-dir web\dist

echo.
echo Feed AI server stopped.
pause
