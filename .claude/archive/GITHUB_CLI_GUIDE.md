# GitHub CLI Quick Reference

## Initial Setup (One Time)

### 1. Authenticate
```bash
gh auth login
```

Follow prompts:
- Choose: **GitHub.com**
- Protocol: **HTTPS** (recommended)
- Authenticate: **Login with a web browser**
- Copy code, press Enter, paste in browser

### 2. Verify Authentication
```bash
gh auth status
```

---

## Using the Helper Script (Easiest)

```powershell
# In PowerShell, navigate to project directory
cd C:\Users\adm.ParisF\acmedns\acme-dns

# Run the helper script
.\gh-helper.ps1
```

**Menu Options**:
1. Push commits to GitHub
2. View workflow runs
3. Watch latest workflow run (real-time)
4. View workflow logs
5. Trigger workflow manually
6. Check repository status
7. View GHCR packages

---

## Manual Commands

### Push Commits
```bash
git push origin master
```

### View Workflow Runs
```bash
# List recent runs
gh run list

# Limit to 5 most recent
gh run list --limit 5

# Filter by status
gh run list --status completed
gh run list --status in_progress
gh run list --status failure
```

### Watch Workflow (Real-time)
```bash
# Watch the most recent run
gh run watch

# Watch a specific run
gh run watch <run-id>
```

### View Workflow Logs
```bash
# View latest run
gh run view

# View specific run with logs
gh run view <run-id> --log

# View specific job logs
gh run view <run-id> --log --job <job-id>
```

### Trigger Workflow Manually
```bash
# Trigger the docker-publish workflow
gh workflow run docker-publish.yml

# Trigger on specific branch
gh workflow run docker-publish.yml --ref master
```

### View Repository Info
```bash
# View repository details
gh repo view

# View in browser
gh repo view --web

# View workflows
gh workflow list

# View specific workflow
gh workflow view docker-publish.yml
```

### Manage Packages (GHCR)
```bash
# List packages (requires authentication)
gh api /user/packages?package_type=container | jq

# Open packages page in browser
# Go to: https://github.com/paz?tab=packages
```

---

## Workflow Status Commands

### Check if Workflow is Running
```bash
gh run list --status in_progress
```

### Check Last Run Status
```bash
gh run list --limit 1
```

### Download Run Logs
```bash
gh run download <run-id>
```

### Cancel a Running Workflow
```bash
gh run cancel <run-id>
```

### Rerun a Failed Workflow
```bash
gh run rerun <run-id>
```

---

## Common Workflows

### After Making Changes

```bash
# 1. Check status
git status

# 2. Commit changes
git add .
git commit -m "Your message"

# 3. Push to GitHub
git push origin master

# 4. Watch the workflow
gh run watch

# 5. View logs if needed
gh run view --log
```

### Monitoring GHCR Build

```bash
# 1. Push commits
git push origin master

# 2. List runs to get the ID
gh run list --limit 1

# 3. Watch it
gh run watch

# 4. Once complete, check packages
# Open: https://github.com/paz?tab=packages

# 5. Pull the image
docker pull ghcr.io/paz/acme-dns:latest
```

### Debugging Failed Workflow

```bash
# 1. List recent runs
gh run list --limit 5

# 2. View failed run logs
gh run view <failed-run-id> --log

# 3. Download logs for detailed inspection
gh run download <failed-run-id>

# 4. Fix the issue, commit, push

# 5. Or manually trigger again
gh workflow run docker-publish.yml
```

---

## Useful Filters and Options

### Filter Runs by Workflow
```bash
gh run list --workflow=docker-publish.yml
```

### View Only Failed Runs
```bash
gh run list --status failure --limit 10
```

### JSON Output (for scripting)
```bash
gh run list --json databaseId,displayTitle,status,conclusion
```

### Format Output
```bash
gh run list --json databaseId,displayTitle,status | jq '.[] | "\(.displayTitle): \(.status)"'
```

---

## GitHub Actions URLs

- **Actions Dashboard**: https://github.com/paz/acme-dns/actions
- **Workflows**: https://github.com/paz/acme-dns/actions/workflows
- **Docker Publish Workflow**: https://github.com/paz/acme-dns/actions/workflows/docker-publish.yml
- **Packages**: https://github.com/paz?tab=packages
- **GHCR Package**: https://github.com/paz/acme-dns/pkgs/container/acme-dns

---

## Environment Variables

```bash
# Set default repository (optional, to avoid specifying -R each time)
export GH_REPO="paz/acme-dns"

# Use specific GitHub Enterprise instance
export GH_HOST="github.com"
```

---

## Aliases (Optional)

Add to your PowerShell profile (`$PROFILE`):

```powershell
# Quick aliases
function ghrl { gh run list @args }
function ghrw { gh run watch @args }
function ghrv { gh run view @args }
function ghwf { gh workflow run @args }
```

Then use:
```powershell
ghrl              # List runs
ghrw              # Watch latest
ghrv --log        # View logs
ghwf docker-publish.yml  # Trigger workflow
```

---

## Troubleshooting

### "gh: command not found"
- Restart your terminal to reload PATH
- Or use full path: `"C:\Program Files\GitHub CLI\gh.exe"`

### "Not logged in"
```bash
gh auth login
```

### "Permission denied" on packages
- Ensure you're logged in: `gh auth status`
- Check token permissions: `gh auth refresh -h github.com -s write:packages`

### "Workflow not found"
```bash
# List available workflows
gh workflow list

# Use exact workflow file name
gh workflow run docker-publish.yml
```

---

## Quick Reference Card

| Task | Command |
|------|---------|
| Push to GitHub | `git push origin master` |
| List runs | `gh run list` |
| Watch latest run | `gh run watch` |
| View logs | `gh run view --log` |
| Trigger workflow | `gh workflow run docker-publish.yml` |
| Check auth | `gh auth status` |
| View repo | `gh repo view` |
| Open in browser | `gh repo view --web` |

---

## Next Steps After Push

1. **Push your commits**:
   ```bash
   git push origin master
   ```

2. **Watch the workflow**:
   ```bash
   gh run watch
   ```

3. **Once complete, make package public**:
   - Go to: https://github.com/paz?tab=packages
   - Click `acme-dns`
   - Settings → Change visibility → Public

4. **Test pulling the image**:
   ```bash
   docker pull ghcr.io/paz/acme-dns:latest
   ```

5. **Deploy to your Linux container**!

---

**For the easiest experience, use the helper script**: `.\gh-helper.ps1`
