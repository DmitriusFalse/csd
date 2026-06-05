@echo off
chcp 65001 >nul
echo ===== Civitai Smart Downloader - Build =====

REM Read and increment build number
if not exist buildnum.txt (
    echo 0 > buildnum.txt
)
set /p CSD_BUILD_NUMBER=<buildnum.txt
set /a CSD_BUILD_NUMBER+=1
echo %CSD_BUILD_NUMBER% > buildnum.txt
set "CSD_BUILD_VERSION=1.1.0.%CSD_BUILD_NUMBER%"
echo Build number: %CSD_BUILD_NUMBER%
echo Build version: %CSD_BUILD_VERSION%

echo --- 1. Server (csd-bridge.exe) ---
go build -ldflags="-H=windowsgui -X main.appVersion=%CSD_BUILD_VERSION%" -o csd-bridge.exe .\cmd\bridge\
if %ERRORLEVEL% neq 0 (
    echo [FAIL] Server build failed!
    exit /b 1
)
echo [OK] server built (version %CSD_BUILD_VERSION%)

echo --- 2. Extension ---
cd extension
set "CSD_BUILD_NUMBER=%CSD_BUILD_NUMBER%"
call npm run build
if %ERRORLEVEL% neq 0 (
    echo [FAIL] Extension build failed!
    cd ..
    exit /b 1
)
cd ..
echo [OK] extension built

echo ===== Build complete, build %CSD_BUILD_NUMBER% =====
