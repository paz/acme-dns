# GitHub CLI Helper Script for acme-dns
# Run this in PowerShell after authenticating with: gh auth login

Write-Host "=== GitHub CLI Helper for acme-dns ===" -ForegroundColor Cyan
Write-Host ""

# Check if authenticated
$authStatus = gh auth status 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "Not authenticated. Please run: gh auth login" -ForegroundColor Red
    exit 1
}

Write-Host "✓ Authenticated" -ForegroundColor Green
Write-Host ""

# Show menu
Write-Host "What would you like to do?" -ForegroundColor Yellow
Write-Host "1. Push commits to GitHub"
Write-Host "2. View workflow runs"
Write-Host "3. Watch latest workflow run (real-time)"
Write-Host "4. View workflow logs"
Write-Host "5. Trigger workflow manually"
Write-Host "6. Check repository status"
Write-Host "7. View GHCR packages"
Write-Host "0. Exit"
Write-Host ""

$choice = Read-Host "Enter choice (0-7)"

switch ($choice) {
    "1" {
        Write-Host "`nPushing commits to origin/master..." -ForegroundColor Yellow
        git push origin master
        if ($LASTEXITCODE -eq 0) {
            Write-Host "`n✓ Pushed successfully!" -ForegroundColor Green
            Write-Host "`nWorkflow will start automatically. View at:" -ForegroundColor Cyan
            Write-Host "  https://github.com/paz/acme-dns/actions" -ForegroundColor Blue
            Write-Host "`nRun option 3 to watch the workflow in real-time." -ForegroundColor Yellow
        }
    }
    "2" {
        Write-Host "`nFetching workflow runs..." -ForegroundColor Yellow
        gh run list --limit 10
    }
    "3" {
        Write-Host "`nWatching latest workflow run..." -ForegroundColor Yellow
        Write-Host "Press Ctrl+C to stop watching" -ForegroundColor Gray
        gh run watch
    }
    "4" {
        Write-Host "`nFetching recent runs..." -ForegroundColor Yellow
        $runs = gh run list --limit 5 --json databaseId,displayTitle,status,conclusion | ConvertFrom-Json

        if ($runs.Count -eq 0) {
            Write-Host "No workflow runs found." -ForegroundColor Yellow
        } else {
            Write-Host "`nRecent workflow runs:" -ForegroundColor Cyan
            for ($i = 0; $i -lt $runs.Count; $i++) {
                $run = $runs[$i]
                $status = if ($run.conclusion) { $run.conclusion } else { $run.status }
                $color = switch ($status) {
                    "success" { "Green" }
                    "failure" { "Red" }
                    "in_progress" { "Yellow" }
                    default { "Gray" }
                }
                Write-Host "  [$($i+1)] $($run.displayTitle) - " -NoNewline
                Write-Host "$status" -ForegroundColor $color
            }

            Write-Host ""
            $selection = Read-Host "Enter number to view logs (or Enter to skip)"

            if ($selection -match '^\d+$' -and [int]$selection -le $runs.Count -and [int]$selection -gt 0) {
                $selectedRun = $runs[[int]$selection - 1]
                Write-Host "`nFetching logs for run $($selectedRun.databaseId)..." -ForegroundColor Yellow
                gh run view $selectedRun.databaseId --log
            }
        }
    }
    "5" {
        Write-Host "`nTriggering 'Docker Build and Push to GHCR' workflow..." -ForegroundColor Yellow
        gh workflow run docker-publish.yml
        if ($LASTEXITCODE -eq 0) {
            Write-Host "✓ Workflow triggered successfully!" -ForegroundColor Green
            Start-Sleep -Seconds 2
            Write-Host "`nFetching run status..." -ForegroundColor Yellow
            gh run list --limit 1
        }
    }
    "6" {
        Write-Host "`nRepository Status:" -ForegroundColor Yellow
        Write-Host ""

        # Git status
        Write-Host "Git Status:" -ForegroundColor Cyan
        git status -sb

        Write-Host ""
        Write-Host "Commits ahead of origin:" -ForegroundColor Cyan
        git log origin/master..HEAD --oneline

        Write-Host ""
        Write-Host "Remote repository:" -ForegroundColor Cyan
        gh repo view --json nameWithOwner,url,visibility | ConvertFrom-Json | Format-List
    }
    "7" {
        Write-Host "`nGitHub Container Registry Packages:" -ForegroundColor Yellow
        Write-Host "Opening packages page in browser..." -ForegroundColor Gray
        Start-Process "https://github.com/paz?tab=packages"

        Write-Host "`nTo pull the image once published:" -ForegroundColor Cyan
        Write-Host "  docker pull ghcr.io/paz/acme-dns:latest" -ForegroundColor Green
    }
    "0" {
        Write-Host "Goodbye!" -ForegroundColor Cyan
        exit 0
    }
    default {
        Write-Host "Invalid choice." -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "Done!" -ForegroundColor Cyan
