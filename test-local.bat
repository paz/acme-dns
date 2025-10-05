@echo off
REM Quick test script for local development
REM This script runs specific test categories

setlocal enabledelayedexpansion

set GO_BIN="C:\Program Files\Go\bin\go.exe"

echo ========================================
echo acme-dns Local Test Runner
echo ========================================
echo.

if "%~1"=="" (
    echo Usage: test-local.bat [option]
    echo.
    echo Options:
    echo   validation    - Run validation tests ^(no DB required^)
    echo   api           - Run API tests ^(requires CGO/MinGW^)
    echo   db            - Run database tests ^(requires CGO/MinGW^)
    echo   dns           - Run DNS tests
    echo   all           - Run all tests ^(requires CGO/MinGW^)
    echo   check-cgo     - Check if CGO is available
    echo.
    exit /b 0
)

if "%~1"=="check-cgo" (
    echo Checking CGO availability...
    echo.
    gcc --version 2>nul
    if errorlevel 1 (
        echo [X] GCC not found - CGO not available
        echo.
        echo SQLite tests will fail without CGO support.
        echo Install MinGW-w64 to enable CGO:
        echo   https://github.com/msys2/msys2-installer/releases
        echo.
        exit /b 1
    ) else (
        echo [✓] GCC found - CGO available
        echo.
        set CGO_ENABLED=1
        %GO_BIN% test -run=^$ ./... 2>&1 | findstr /C:"CGO_ENABLED"
        echo.
        echo CGO is ready for SQLite tests!
        exit /b 0
    )
)

if "%~1"=="validation" (
    echo Running validation tests ^(no DB required^)...
    echo.
    set CGO_ENABLED=0
    %GO_BIN% test -v -run "TestValid|TestGetValid|TestCorrectPassword|TestGetIPList|TestPrepareConfig" ./...
    exit /b !errorlevel!
)

if "%~1"=="dns" (
    echo Running DNS tests...
    echo.
    set CGO_ENABLED=0
    %GO_BIN% test -v -run "TestResolve|TestEDNS|TestParse|TestAuthoritative|TestCaseInsensitive" ./...
    exit /b !errorlevel!
)

if "%~1"=="api" (
    echo Running API tests ^(requires CGO^)...
    echo.
    set CGO_ENABLED=1
    %GO_BIN% test -v -run "TestApi" ./...
    exit /b !errorlevel!
)

if "%~1"=="db" (
    echo Running database tests ^(requires CGO^)...
    echo.
    set CGO_ENABLED=1
    %GO_BIN% test -v -run "TestDB|TestRegister|TestGetByUsername|TestGetTXT|TestUpdate" ./...
    exit /b !errorlevel!
)

if "%~1"=="all" (
    echo Running ALL tests ^(requires CGO^)...
    echo.
    set CGO_ENABLED=1
    %GO_BIN% test -v ./...
    exit /b !errorlevel!
)

echo Unknown option: %~1
echo Run without arguments to see usage.
exit /b 1

endlocal
