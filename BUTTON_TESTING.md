# Button Functionality Testing Guide - Quick Reference

## Prerequisites
- acme-dns running with web UI enabled
- Admin account created: `./acme-dns --create-admin admin@test.com`
- Login at http://localhost:port/login

## Test 1: Create User Button

**Location:** Admin page → Users tab → "Create User" button

1. Click "Create User"
2. Fill form: email, password (12+ chars), optional admin checkbox
3. Submit
4. **Expected:** Green toast "User created successfully", page reloads, new user in table

**Error Test:** Duplicate email → Red toast with error message

## Test 2: Claim Domain Button

**Location:** Admin page → Unmanaged Domains tab → "Claim for User" button

1. Create API domain first: `curl -X POST http://localhost:port/register`
2. In admin page, click "Claim for User" on unmanaged domain
3. Select user from dropdown
4. Submit
5. **Expected:** Green toast "Domain claimed successfully", domain moves to managed list

## Test 3: Revoke Session Button

**Location:** Profile page → Active Sessions → "Revoke" button

1. Login from 2 different browsers (creates 2 sessions)
2. In profile, click "Revoke" on non-current session
3. Confirm
4. **Expected:** Green toast "Session revoked successfully", session removed from list

## Browser Console Check
- Open DevTools (F12) → Console
- Should see no red errors
- Network tab should show JSON responses (not plain text)

## Success Response Format
```json
{"status": "success", "user": {...}}
```

## Error Response Format
```json
{"status": "error", "message": "..."}
```
