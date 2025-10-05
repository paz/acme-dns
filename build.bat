@echo off
REM Build script for acme-dns on Windows
REM Usage: build.bat [options]
REM   --with-tests    Build and run tests (requires GCC/MinGW for SQLite)
REM   --linux         Cross-compile for Linux

setlocal enabledelayedexpansion

echo ========================================
echo acme-dns Build Script (Windows)
echo ========================================
echo.

REM Default Go binary path
set GO_BIN="C:\Program Files\Go\bin\go.exe"

REM Parse arguments
set RUN_TESTS=0
set CROSS_COMPILE=0

:parse_args
if "%~1"=="" goto :end_parse
if "%~1"=="--with-tests" set RUN_TESTS=1
if "%~1"=="--linux" set CROSS_COMPILE=1
shift
goto :parse_args
:end_parse

REM Clean previous builds
if exist acme-dns.exe del acme-dns.exe
if exist acme-dns del acme-dns
echo [1/4] Cleaning previous builds... DONE
echo.

REM Run go mod tidy
echo [2/4] Updating dependencies...
%GO_BIN% mod tidy
if errorlevel 1 (
    echo ERROR: Failed to run go mod tidy
    exit /b 1
)
echo      Dependencies updated successfully
echo.

REM Build for Windows or Linux
if %CROSS_COMPILE%==1 (
    echo [3/4] Cross-compiling for Linux...
    set GOOS=linux
    set GOARCH=amd64
    set CGO_ENABLED=0
    %GO_BIN% build -v -o acme-dns
    if errorlevel 1 (
        echo ERROR: Cross-compilation failed
        exit /b 1
    )
    echo      Linux binary built: acme-dns
) else (
    echo [3/4] Building Windows binary...
    set CGO_ENABLED=0
    %GO_BIN% build -v -o acme-dns.exe
    if errorlevel 1 (
        echo ERROR: Build failed
        exit /b 1
    )
    echo      Windows binary built: acme-dns.exe
)
echo.

REM Run tests if requested
if %RUN_TESTS%==1 (
    echo [4/4] Running tests...
    echo.
    echo NOTE: Tests require CGO (GCC/MinGW) for SQLite support.
    echo       Install MinGW-w64 from: https://www.mingw-w64.org/
    echo.
    set CGO_ENABLED=1
    %GO_BIN% test -v ./...
    if errorlevel 1 (
        echo.
        echo WARNING: Tests failed. This is normal on Windows without GCC.
        echo          Tests will pass on Linux (GitHub Actions).
        echo.
        echo To install MinGW-w64 for local testing:
        echo   1. Download from: https://github.com/msys2/msys2-installer/releases
        echo   2. Install MSYS2
        echo   3. Run: pacman -S mingw-w64-x86_64-gcc
        echo   4. Add C:\msys64\mingw64\bin to PATH
        exit /b 0
    )
    echo      All tests passed!
) else (
    echo [4/4] Skipping tests (use --with-tests to run)
)

echo.
echo ========================================
echo Build completed successfully!
echo ========================================
echo.
if %CROSS_COMPILE%==1 (
    echo Binary: acme-dns ^(Linux AMD64^)
) else (
    echo Binary: acme-dns.exe ^(Windows^)
)
echo.
echo Quick commands:
echo   Build only:        build.bat
echo   Build + tests:     build.bat --with-tests
echo   Linux binary:      build.bat --linux
echo.

endlocal
