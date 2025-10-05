# Local Development Guide - acme-dns

## Quick Start (Windows)

### Prerequisites
- Go 1.22+ installed at `C:\Program Files\Go`
- Git for version control
- (Optional) MinGW-w64 for SQLite test support
- (Optional) golangci-lint for local linting

### Fast Development Cycle

#### 1. Build Only (Fastest - No Tests)
```cmd
build.bat
```
**Time:** ~5-10 seconds
**Output:** `acme-dns.exe`

#### 2. Build with Validation Tests (No DB)
```cmd
test-local.bat validation
```
**Time:** ~10-15 seconds
**Tests:** Validation, password, config tests (no SQLite needed)

#### 3. Build with All Tests (Requires MinGW)
```cmd
build.bat --with-tests
```
**Time:** ~30-60 seconds
**Requires:** GCC compiler (MinGW-w64)

#### 4. Cross-Compile for Linux
```cmd
build.bat --linux
```
**Time:** ~5-10 seconds
**Output:** `acme-dns` (Linux binary)

### Test Categories

Run specific test groups without full test suite:

```cmd
# Validation tests (no DB) - FASTEST
test-local.bat validation

# DNS tests (no DB)
test-local.bat dns

# API tests (requires CGO)
test-local.bat api

# Database tests (requires CGO)
test-local.bat db

# All tests
test-local.bat all

# Check CGO availability
test-local.bat check-cgo
```

### Install MinGW-w64 (for SQLite/CGO Support)

**Why needed?** SQLite requires CGO, which needs a C compiler on Windows.

**Quick Install:**
1. Download MSYS2: https://github.com/msys2/msys2-installer/releases
2. Install to default location (`C:\msys64`)
3. Run MSYS2 terminal and execute:
   ```bash
   pacman -S mingw-w64-x86_64-gcc
   ```
4. Add to PATH (System Environment Variables):
   ```
   C:\msys64\mingw64\bin
   ```
5. Restart terminal and verify:
   ```cmd
   gcc --version
   ```

### Linting Locally

```cmd
# Run golangci-lint
lint.bat
```

**Install golangci-lint:**
```cmd
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

Or download from: https://github.com/golangci/golangci-lint/releases

### Recommended Workflow

#### Option 1: Fast Iteration (No Tests)
```cmd
# Edit code
build.bat
# Test manually or push to GitHub for full CI
git add .
git commit -m "Your changes"
git push
```

#### Option 2: With Validation Tests
```cmd
# Edit code
test-local.bat validation
build.bat
# If validation passes, push
git add .
git commit -m "Your changes"
git push
```

#### Option 3: Full Local Testing (with MinGW)
```cmd
# Edit code
build.bat --with-tests
# All tests pass locally
git add .
git commit -m "Your changes"
git push
```

### Common Commands

```cmd
# Quick build
build.bat

# Build + validation tests (fast, no DB)
test-local.bat validation && build.bat

# Build + all tests (requires MinGW)
build.bat --with-tests

# Check your code style
lint.bat

# Cross-compile for Linux deployment
build.bat --linux

# Run specific DNS tests
test-local.bat dns

# Check if CGO is available
test-local.bat check-cgo
```

## GitHub Actions vs Local Testing

### GitHub Actions (Always Run)
- ✅ Full test suite with CGO
- ✅ golangci-lint
- ✅ CodeQL security scanning
- ✅ Docker multi-platform builds
- ⏱️ Takes 3-5 minutes total

### Local Testing (Your Choice)
- ⚡ **Fast:** `build.bat` (5 sec) - Build only
- ⚡ **Medium:** `test-local.bat validation` (15 sec) - Basic tests
- 🐢 **Full:** `build.bat --with-tests` (60 sec) - All tests (needs MinGW)

### Recommendation
1. **During development:** Use `build.bat` or `test-local.bat validation`
2. **Before committing:** Run `lint.bat` if you have it installed
3. **Let GitHub Actions:** Handle full testing and Docker builds

## CI/CD Pipeline Status

When you push, these workflows run automatically:

1. **Go Tests** (~3 min) - Full test suite with coverage
2. **golangci-lint** (~1 min) - Code quality checks
3. **CodeQL** (~2 min) - Security analysis
4. **Docker Build** (~15 min) - Multi-platform containers

Check status:
```cmd
gh run list --limit 5
gh run watch
```

## Troubleshooting

### "Tests fail on Windows"
- **Normal!** SQLite tests need CGO (MinGW compiler)
- Tests will pass on Linux (GitHub Actions)
- Install MinGW-w64 for local testing (see above)

### "Build fails with import errors"
```cmd
go mod tidy
build.bat
```

### "CGO not available"
```cmd
test-local.bat check-cgo
```
Follow the MinGW installation steps above.

### "Linter not found"
```cmd
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

## Development Tips

### 1. Fast Feedback Loop
```cmd
# Keep this running in one terminal
build.bat && echo Success || echo Failed
```

### 2. Watch Mode (PowerShell)
```powershell
# Auto-rebuild on file changes
while($true) {
    $w = [System.IO.File]::GetLastWriteTime(".")
    Start-Sleep -Seconds 1
    if([System.IO.File]::GetLastWriteTime(".") -gt $w) {
        build.bat
    }
}
```

### 3. Quick Test Specific Function
```cmd
go test -v -run TestSpecificFunction ./...
```

### 4. Check What Will Run in CI
```cmd
# Simulate GitHub Actions locally
set CGO_ENABLED=1
go test -v -race -covermode=atomic ./...
```

## Performance Comparison

| Method | Time | Tests | CGO Required |
|--------|------|-------|--------------|
| `build.bat` | 5-10s | No | No |
| `test-local.bat validation` | 10-15s | Yes (basic) | No |
| `test-local.bat dns` | 15-20s | Yes (DNS only) | No |
| `build.bat --with-tests` | 30-60s | Yes (all) | Yes |
| GitHub Actions | 3-5min | Yes (all + lint + security) | Yes |

## Files Reference

- `build.bat` - Main build script
- `test-local.bat` - Selective test runner
- `lint.bat` - Local linting
- `config.cfg` - Configuration file
- `.github/workflows/` - CI/CD definitions

## Next Steps

1. Install MinGW-w64 for full local testing (optional)
2. Install golangci-lint for code quality checks (optional)
3. Use `build.bat` for fast iterations
4. Let GitHub Actions handle comprehensive testing
5. Check workflow status with `gh run list`

---

**Happy coding! 🚀**

For questions or issues, check:
- CLAUDE.md - Project architecture guide
- README.md - General documentation
- GitHub Issues - Known problems and discussions
