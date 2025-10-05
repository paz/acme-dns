# Template Rendering Issue - Fix Required

## Problem

All pages are showing the login page because the template inheritance is broken.

## Root Cause

Template files like `login-page.html`, `dashboard-page.html`, and `admin-page.html` have:
```
{{template "base" .}}

{{define "content"}}
...
{{end}}
```

The `{{template "base" .}}` on line 1 immediately executes the base template, which then tries to render `{{template "content" .}}` creating a rendering loop where only the first template's content is shown.

## Solution

### 1. Template Files Should ONLY Define Content Blocks

Each template file should ONLY define its content block, no execute statements:

**login.html:**
```go
{{define "login-content"}}
<div class="row justify-content-center">
    <!-- login form here -->
</div>
{{end}}
```

**dashboard.html:**
```go
{{define "dashboard-content"}}
<div class="row">
    <!-- dashboard UI here -->
</div>
{{end}}
```

**admin.html:**
```go
{{define "admin-content"}}
<div class="row">
    <!-- admin panel here -->
</div>
{{end}}
```

**profile.html:**
```go
{{define "profile-content"}}
<div class="row">
    <!-- profile form here -->
</div>
{{end}}
```

### 2. Base Template Calls Specific Content

**layout.html** (already updated):
```go
{{define "base"}}
<!DOCTYPE html>
...
<main class="container mt-4">
    {{if .Data.ContentTemplate}}
        {{template .Data.ContentTemplate .}}
    {{else}}
        {{template "content" .}}
    {{end}}
</main>
...
{{end}}
```

### 3. Handlers Use render() Helper

**web/handlers.go** already has:
```go
func (h *Handlers) render(w http.ResponseWriter, contentTemplate string, data *TemplateData) error {
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    data.Data["ContentTemplate"] = contentTemplate
    return h.templates.ExecuteTemplate(w, "base", data)
}
```

Each handler should call:
```go
h.render(w, "login-content", data)       // for login page
h.render(w, "dashboard-content", data)   // for dashboard
h.render(w, "admin-content", data)       // for admin panel
h.render(w, "profile-content", data)     // for profile page
```

## Files That Need Fixing

### Templates to Update:
- [x] `web/templates/login.html` - Already fixed to use `{{define "login-content"}}`
- [ ] `web/templates/dashboard.html` - Need to rename from `dashboard-page.html` and use `{{define "dashboard-content"}}`
- [ ] `web/templates/admin.html` - Need to rename from `admin-page.html` and use `{{define "admin-content"}}`
- [ ] `web/templates/profile.html` - **MISSING** - Needs to be created
- [ ] `web/templates/register.html` - **MISSING** - Needs to be created (if self-registration enabled)

### Handlers to Update:
- [x] `web/handlers.go` - LoginPage() - Already uses `h.render(w, "login-content", data)`
- [ ] `web/handlers.go` - Dashboard() - Change to `h.render(w, "dashboard-content", data)`
- [ ] `web/handlers.go` - Profile() - **MISSING HANDLER** - Needs to be created
- [ ] `web/handlers.go` - RegisterPage() - **MISSING HANDLER** - Needs to be created
- [ ] `admin/handlers.go` - AdminPanel() - Change to `h.render(w, "admin-content", data)`

### Routes to Add in main.go:
- [ ] GET /profile - ProfilePage handler
- [ ] POST /profile - ProfileUpdate handler
- [ ] GET /register - RegisterPage handler (if allow_self_registration)
- [ ] POST /register - RegisterPost handler (if allow_self_registration)

## Quick Fix Commands

### 1. Delete problematic template files:
```bash
rm web/templates/login-page.html
rm web/templates/dashboard-page.html
rm web/templates/admin-page.html
rm web/templates/base.html  # We're using layout.html instead
```

### 2. Update Dashboard Handler:
In `web/handlers.go`, find `Dashboard()` function and change:
```go
// OLD:
w.Header().Set("Content-Type", "text/html; charset=utf-8")
if err := h.templates.ExecuteTemplate(w, "dashboard-page.html", data); err != nil {

// NEW:
if err := h.render(w, "dashboard-content", data); err != nil {
```

### 3. Update Admin Handler:
In `admin/handlers.go`, find `AdminPanel()` function and change:
```go
// OLD:
w.Header().Set("Content-Type", "text/html; charset=utf-8")
if err := h.templates.ExecuteTemplate(w, "admin-page.html", data); err != nil {

// NEW:
if err := h.render(w, "admin-content", data); err != nil {
```

But admin handlers don't have render() helper, so need to add it or use web.render somehow.

## Missing Features

### 1. Profile Page (Currently 404)
Users click "Profile" in navigation but page doesn't exist.

**Need to create:**
- `web/templates/profile.html` with `{{define "profile-content"}}`
- Handler in `web/handlers.go`:
  ```go
  func (h *Handlers) ProfilePage(w http.ResponseWriter, r *http.Request, _ httprouter.Params)
  func (h *Handlers) ProfileUpdate(w http.ResponseWriter, r *http.Request, _ httprouter.Params)
  ```
- Routes in `main.go`:
  ```go
  api.GET("/profile", web.ChainMiddleware(webHandlers.ProfilePage, ...))
  api.POST("/profile", web.ChainMiddleware(webHandlers.ProfileUpdate, ...))
  ```

### 2. Registration Page (If Enabled)
If `allow_self_registration = true`, need registration page.

**Need to create:**
- `web/templates/register.html` with `{{define "register-content"}}`
- Handler in `web/handlers.go`:
  ```go
  func (h *Handlers) RegisterPage(w http.ResponseWriter, r *http.Request, _ httprouter.Params)
  func (h *Handlers) RegisterPost(w http.ResponseWriter, r *http.Request, _ httprouter.Params)
  ```

### 3. Admin Helper Function
`admin/handlers.go` needs the same `render()` helper as web/handlers.go

## Testing After Fix

1. **Login Page:**
   ```
   curl https://your-server/login
   ```
   Should show full HTML with login form, NOT just login form without nav/footer

2. **Dashboard After Login:**
   Should show dashboard content with navigation bar at top

3. **Admin Panel:**
   Should show admin panel with navigation bar

4. **Profile Page:**
   Should NOT 404, should show profile form

## Current Status

- ✅ Base template (layout.html) supports ContentTemplate
- ✅ Login template fixed to use "login-content"
- ✅ Login handler uses render() helper
- ✅ render() helper function exists
- ❌ Dashboard template still broken (dashboard-page.html)
- ❌ Admin template still broken (admin-page.html)
- ❌ Profile page completely missing
- ❌ Registration page completely missing
- ❌ Dashboard handler not using render()
- ❌ Admin handler not using render()
- ❌ Admin handlers don't have render() helper

## Recommended Approach

1. Create all missing templates with correct naming
2. Update all handlers to use render() pattern
3. Add render() helper to admin/handlers.go
4. Add missing Profile and Register handlers
5. Add missing routes in main.go
6. Test each page individually
7. Commit and deploy

## Files Reference

- `web/templates/layout.html` - Base template (✅ correct)
- `web/templates/login.html` - Login content (✅ correct)
- `web/templates/dashboard.html` - Dashboard content (❌ needs fix)
- `web/templates/admin.html` - Admin content (❌ needs fix)
- `web/templates/profile.html` - Profile content (❌ missing)
- `web/templates/register.html` - Registration content (❌ missing)
- `web/handlers.go` - Web handlers
- `admin/handlers.go` - Admin handlers
- `main.go` - Route definitions
