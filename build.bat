@echo off
chcp 65001 >nul
echo ===== Civitai Smart Downloader - Build =====

echo --- 1. Server (csd-bridge.exe) ---
go build -ldflags="-H=windowsgui" -o csd-bridge.exe .\cmd\bridge\
if %ERRORLEVEL% neq 0 (
    echo [FAIL] Server build failed!
    exit /b 1
)
echo [OK] server built

echo --- 2. Extension ---
cd extension
call npm run build
if %ERRORLEVEL% neq 0 (
    echo [FAIL] Extension build failed!
    cd ..
    exit /b 1
)
cd ..
echo [OK] extension built

echo ===== Build complete =====
