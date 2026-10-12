@echo off
rem Starts the Feed AI server from the package folder.
cd /d "%~dp0"

server.exe -addr :8080 -content-dir test-content -static-dir web\dist

echo.
echo Feed AI server stopped.
pause
