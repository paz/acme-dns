@echo off
REM Run golangci-lint locally (if installed)

setlocal

echo ========================================
echo Running golangci-lint
echo ========================================
echo.

REM Check if golangci-lint is installed
where golangci-lint >nul 2>&1
if errorlevel 1 (
    echo golangci-lint is not installed.
    echo.
    echo To install:
    echo   1. Download from: https://github.com/golangci/golangci-lint/releases
    echo   2. Or use: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    echo   3. Ensure it's in your PATH
    echo.
    exit /b 1
)

echo Running linter with 10-minute timeout...
echo.

golangci-lint run --timeout=10m

if errorlevel 1 (
    echo.
    echo Linting found issues. Please fix before pushing.
    exit /b 1
) else (
    echo.
    echo ✓ All checks passed!
    exit /b 0
)

endlocal
