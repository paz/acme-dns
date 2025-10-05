// Get CSRF token from meta tag
function getCSRFToken() {
    const meta = document.querySelector('meta[name="csrf-token"]');
    return meta ? meta.getAttribute('content') : '';
}

// Cache the CSRF token on page load
let csrfToken = '';
document.addEventListener('DOMContentLoaded', () => {
    csrfToken = getCSRFToken();
});

// Helper function to copy text to clipboard
function copyToClipboard(elementId) {
    const element = document.getElementById(elementId);
    element.select();
    element.setSelectionRange(0, 99999); // For mobile devices

    navigator.clipboard.writeText(element.value).then(() => {
        showToast('Copied to clipboard!', 'success');
    }).catch(err => {
        console.error('Failed to copy:', err);
        showToast('Failed to copy to clipboard', 'danger');
    });
}

// Show toast notification
function showToast(message, type = 'info') {
    const toastContainer = document.getElementById('toast-container') || createToastContainer();

    const toast = document.createElement('div');
    toast.className = `toast align-items-center text-white bg-${type} border-0`;
    toast.setAttribute('role', 'alert');
    toast.innerHTML = `
        <div class="d-flex">
            <div class="toast-body">${message}</div>
            <button type="button" class="btn-close btn-close-white me-2 m-auto" data-bs-dismiss="toast"></button>
        </div>
    `;

    toastContainer.appendChild(toast);
    const bsToast = new bootstrap.Toast(toast);
    bsToast.show();

    // Remove toast element after it's hidden
    toast.addEventListener('hidden.bs.toast', () => {
        toast.remove();
    });
}

function createToastContainer() {
    const container = document.createElement('div');
    container.id = 'toast-container';
    container.className = 'toast-container position-fixed bottom-0 end-0 p-3';
    document.body.appendChild(container);
    return container;
}

// Dashboard functions - View credentials
function viewCredentials(username) {
    const modal = new bootstrap.Modal(document.getElementById('credentialsModal'));
    modal.show();

    fetch('/dashboard/domain/' + encodeURIComponent(username) + '/credentials')
        .then(r => r.json())
        .then(data => {
            const container = document.getElementById('credentialsContent');
            container.innerHTML = ''; // Clear first

            // Build DOM safely without innerHTML to prevent XSS
            const warning = document.createElement('div');
            warning.className = 'alert alert-warning';
            warning.innerHTML = '<i class="bi bi-exclamation-triangle"></i> <strong>Keep these credentials secure!</strong> They cannot be retrieved again.';
            container.appendChild(warning);

            // Username field
            container.appendChild(createCredentialField('Username', data.username));
            // Password field
            container.appendChild(createCredentialField('Password', data.password));
            // Full domain field
            container.appendChild(createCredentialField('Full Domain', data.fulldomain));
        })
        .catch(err => {
            document.getElementById('credentialsContent').textContent = 'Error loading credentials';
        });
}

function createCredentialField(label, value) {
    const div = document.createElement('div');
    div.className = 'mb-3';

    const labelEl = document.createElement('label');
    labelEl.className = 'form-label';
    labelEl.textContent = label;
    div.appendChild(labelEl);

    const inputGroup = document.createElement('div');
    inputGroup.className = 'input-group';

    const input = document.createElement('input');
    input.type = 'text';
    input.className = 'form-control';
    input.value = value; // Safe - direct property assignment
    input.readOnly = true;
    inputGroup.appendChild(input);

    const button = document.createElement('button');
    button.className = 'btn btn-outline-secondary';
    button.innerHTML = '<i class="bi bi-clipboard"></i>';
    button.addEventListener('click', () => {
        navigator.clipboard.writeText(value).then(() => {
            showToast('Copied to clipboard!', 'success');
        }).catch(() => {
            showToast('Failed to copy to clipboard', 'danger');
        });
    });
    inputGroup.appendChild(button);

    div.appendChild(inputGroup);
    return div;
}

function deleteDomain(username) {
    if (!confirm('Are you sure you want to delete this domain? This action cannot be undone.')) {
        return;
    }

    fetch('/dashboard/domain/' + encodeURIComponent(username), {
        method: 'DELETE',
        headers: {
            'X-CSRF-Token': csrfToken
        }
    }).then(() => {
        showToast('Domain deleted successfully', 'success');
        setTimeout(() => window.location.reload(), 1000);
    }).catch(err => {
        showToast('Failed to delete domain', 'danger');
    });
}

// Register domain form handler
document.addEventListener('DOMContentLoaded', () => {
    const registerForm = document.getElementById('registerForm');
    if (registerForm) {
        registerForm.addEventListener('submit', (e) => {
            e.preventDefault();

            const formData = new FormData(registerForm);
            const data = {
                csrf_token: formData.get('csrf_token'),
                description: formData.get('description'),
                allowfrom: formData.get('allowfrom')
            };

            fetch('/dashboard/register', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-CSRF-Token': csrfToken
                },
                body: JSON.stringify(data)
            })
            .then(response => response.json())
            .then(data => {
                if (data.status === 'success') {
                    showToast('Domain registered successfully', 'success');
                    bootstrap.Modal.getInstance(document.getElementById('registerModal')).hide();
                    setTimeout(() => location.reload(), 1000);
                } else {
                    showToast(data.message || 'Failed to register domain', 'danger');
                }
            })
            .catch(error => {
                console.error('Error:', error);
                showToast('Failed to register domain', 'danger');
            });
        });
    }
});

// Admin functions
function deleteUser(userId, email) {
    if (!confirm(`Are you sure you want to delete user ${email}?`)) {
        return;
    }

    fetch(`/admin/users/${userId}`, {
        method: 'DELETE',
        headers: {
            'X-CSRF-Token': csrfToken
        }
    })
    .then(response => response.json())
    .then(data => {
        if (data.status === 'success') {
            showToast('User deleted successfully', 'success');
            setTimeout(() => location.reload(), 1000);
        } else {
            showToast('Failed to delete user', 'danger');
        }
    })
    .catch(error => {
        console.error('Error:', error);
        showToast('Failed to delete user', 'danger');
    });
}

function toggleUserActive(userId, currentlyActive) {
    const action = currentlyActive ? 'disable' : 'enable';

    if (!confirm(`Are you sure you want to ${action} this user?`)) {
        return;
    }

    fetch(`/admin/users/${userId}/toggle`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/x-www-form-urlencoded',
            'X-CSRF-Token': csrfToken
        },
        body: `active=${!currentlyActive}&csrf_token=${csrfToken}`
    })
    .then(response => response.json())
    .then(data => {
        if (data.status === 'success') {
            showToast(`User ${action}d successfully`, 'success');
            setTimeout(() => location.reload(), 1000);
        } else {
            showToast(`Failed to ${action} user`, 'danger');
        }
    })
    .catch(error => {
        console.error('Error:', error);
        showToast(`Failed to ${action} user`, 'danger');
    });
}

function adminDeleteDomain(username, subdomain) {
    if (!confirm(`Are you sure you want to delete ${subdomain}?`)) {
        return;
    }

    fetch(`/admin/domains/${username}`, {
        method: 'DELETE',
        headers: {
            'X-CSRF-Token': csrfToken
        }
    })
    .then(response => response.json())
    .then(data => {
        if (data.status === 'success') {
            showToast('Domain deleted successfully', 'success');
            setTimeout(() => location.reload(), 1000);
        } else {
            showToast('Failed to delete domain', 'danger');
        }
    })
    .catch(error => {
        console.error('Error:', error);
        showToast('Failed to delete domain', 'danger');
    });
}

function showClaimModal(username, subdomain) {
    document.getElementById('claim-username').value = username;
    document.getElementById('claim-subdomain').value = subdomain;
    const modal = new bootstrap.Modal(document.getElementById('claimDomainModal'));
    modal.show();
}

// Create user form handler
document.addEventListener('DOMContentLoaded', () => {
    const createUserForm = document.getElementById('createUserForm');
    if (createUserForm) {
        createUserForm.addEventListener('submit', (e) => {
            e.preventDefault();

            const formData = new FormData(createUserForm);

            fetch('/admin/users', {
                method: 'POST',
                headers: {
                    'X-CSRF-Token': csrfToken
                },
                body: formData
            })
            .then(response => response.json())
            .then(data => {
                if (data.status === 'success') {
                    showToast('User created successfully', 'success');
                    bootstrap.Modal.getInstance(document.getElementById('createUserModal')).hide();
                    createUserForm.reset();
                    setTimeout(() => location.reload(), 1000);
                } else {
                    showToast(data.message || 'Failed to create user', 'danger');
                }
            })
            .catch(error => {
                console.error('Error:', error);
                showToast('Failed to create user', 'danger');
            });
        });
    }
});

// Claim domain form handler
document.addEventListener('DOMContentLoaded', () => {
    const claimForm = document.getElementById('claimDomainForm');
    if (claimForm) {
        claimForm.addEventListener('submit', (e) => {
            e.preventDefault();

            const username = document.getElementById('claim-username').value;
            const formData = new FormData(claimForm);

            fetch(`/admin/claim/${username}`, {
                method: 'POST',
                headers: {
                    'X-CSRF-Token': csrfToken
                },
                body: formData
            })
            .then(response => response.json())
            .then(data => {
                if (data.status === 'success') {
                    showToast('Domain claimed successfully', 'success');
                    bootstrap.Modal.getInstance(document.getElementById('claimDomainModal')).hide();
                    claimForm.reset();
                    setTimeout(() => location.reload(), 1000);
                } else {
                    showToast(data.message || 'Failed to claim domain', 'danger');
                }
            })
            .catch(error => {
                console.error('Error:', error);
                showToast('Failed to claim domain', 'danger');
            });
        });
    }
});

// Bulk claim form handler
document.addEventListener('DOMContentLoaded', () => {
    const bulkClaimForm = document.getElementById('bulkClaimForm');
    if (bulkClaimForm) {
        bulkClaimForm.addEventListener('submit', (e) => {
            e.preventDefault();

            const usernames = JSON.parse(bulkClaimForm.dataset.usernames || '[]');
            const userId = parseInt(document.getElementById('bulk-claim-user-id').value);
            const description = document.getElementById('bulk-claim-description').value;

            if (!userId) {
                showToast('Please select a user', 'warning');
                return;
            }

            fetch('/admin/domains/bulk-claim', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'X-CSRF-Token': csrfToken
                },
                body: JSON.stringify({
                    usernames,
                    user_id: userId,
                    description
                })
            })
            .then(response => response.json())
            .then(data => {
                if (data.status === 'success') {
                    const msg = `Claimed ${data.success_count} of ${data.total} domain(s)`;
                    showToast(msg, data.fail_count > 0 ? 'warning' : 'success');
                    if (data.errors && data.errors.length > 0) {
                        console.error('Bulk claim errors:', data.errors);
                    }
                    bootstrap.Modal.getInstance(document.getElementById('bulkClaimModal')).hide();
                    bulkClaimForm.reset();
                    setTimeout(() => location.reload(), 1500);
                } else {
                    showToast(data.message || 'Failed to claim domains', 'danger');
                }
            })
            .catch(error => {
                console.error('Error:', error);
                showToast('Failed to claim domains', 'danger');
            });
        });
    }
});

// Profile page - Revoke session function
function revokeSession(sessionId) {
    if (!confirm('Are you sure you want to revoke this session?')) {
        return;
    }

    fetch('/profile/sessions/' + sessionId, {
        method: 'DELETE',
        headers: {
            'X-CSRF-Token': csrfToken
        }
    })
    .then(response => response.json())
    .then(data => {
        if (data.status === 'success') {
            showToast('Session revoked successfully', 'success');
            setTimeout(() => window.location.reload(), 1000);
        } else {
            showToast(data.message || 'Failed to revoke session', 'danger');
        }
    })
    .catch(error => {
        console.error('Error:', error);
        showToast('Failed to revoke session', 'danger');
    });
}

// Bulk selection management
function updateBulkActionButtons(table) {
    const checkboxes = document.querySelectorAll(`.domain-checkbox[data-table="${table}"]:checked`);
    const count = checkboxes.length;

    // Update counts
    document.querySelectorAll(`.selected-count-${table}`).forEach(el => {
        el.textContent = count;
    });

    // Enable/disable buttons
    if (table === 'all') {
        const deleteBtn = document.querySelector('.bulk-delete-all-btn');
        if (deleteBtn) {
            deleteBtn.disabled = count === 0;
        }
    } else if (table === 'unmanaged') {
        const claimBtn = document.querySelector('.bulk-claim-btn');
        const deleteBtn = document.querySelector('.bulk-delete-unmanaged-btn');
        if (claimBtn) claimBtn.disabled = count === 0;
        if (deleteBtn) deleteBtn.disabled = count === 0;
    }
}

function getSelectedDomains(table) {
    const checkboxes = document.querySelectorAll(`.domain-checkbox[data-table="${table}"]:checked`);
    return Array.from(checkboxes).map(cb => ({
        username: cb.dataset.username,
        subdomain: cb.dataset.subdomain
    }));
}

function bulkClaimDomains() {
    const selected = getSelectedDomains('unmanaged');
    if (selected.length === 0) {
        showToast('No domains selected', 'warning');
        return;
    }

    // Populate bulk claim modal
    document.getElementById('bulk-claim-count').textContent = selected.length;
    const listEl = document.getElementById('bulk-claim-list');
    listEl.innerHTML = selected.map(d => `<div><code>${d.subdomain}</code></div>`).join('');

    // Store selected usernames for form submission
    document.getElementById('bulkClaimForm').dataset.usernames = JSON.stringify(selected.map(d => d.username));

    // Show modal
    const modal = new bootstrap.Modal(document.getElementById('bulkClaimModal'));
    modal.show();
}

function bulkDeleteDomains(table) {
    const selected = getSelectedDomains(table);
    if (selected.length === 0) {
        showToast('No domains selected', 'warning');
        return;
    }

    if (!confirm(`Are you sure you want to delete ${selected.length} domain(s)? This action cannot be undone.`)) {
        return;
    }

    const usernames = selected.map(d => d.username);

    fetch('/admin/domains/bulk-delete', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'X-CSRF-Token': csrfToken
        },
        body: JSON.stringify({ usernames })
    })
    .then(response => response.json())
    .then(data => {
        if (data.status === 'success') {
            const msg = `Deleted ${data.success_count} of ${data.total} domain(s)`;
            showToast(msg, data.fail_count > 0 ? 'warning' : 'success');
            if (data.errors && data.errors.length > 0) {
                console.error('Bulk delete errors:', data.errors);
            }
            setTimeout(() => location.reload(), 1500);
        } else {
            showToast(data.message || 'Failed to delete domains', 'danger');
        }
    })
    .catch(error => {
        console.error('Error:', error);
        showToast('Failed to delete domains', 'danger');
    });
}

// Event delegation for dynamically added buttons
document.addEventListener('DOMContentLoaded', () => {
    // Dashboard - View credentials buttons
    document.querySelectorAll('.view-credentials').forEach(btn => {
        btn.addEventListener('click', function() {
            viewCredentials(this.dataset.username);
        });
    });

    // Dashboard - Delete domain buttons
    document.querySelectorAll('.delete-domain').forEach(btn => {
        btn.addEventListener('click', function() {
            deleteDomain(this.dataset.username);
        });
    });

    // Select-all checkboxes
    document.querySelectorAll('.select-all-domains').forEach(checkbox => {
        checkbox.addEventListener('change', function() {
            const table = this.dataset.table;
            const checked = this.checked;
            document.querySelectorAll(`.domain-checkbox[data-table="${table}"]`).forEach(cb => {
                cb.checked = checked;
            });
            updateBulkActionButtons(table);
        });
    });

    // Individual domain checkboxes
    document.querySelectorAll('.domain-checkbox').forEach(checkbox => {
        checkbox.addEventListener('change', function() {
            updateBulkActionButtons(this.dataset.table);

            // Update select-all checkbox state
            const table = this.dataset.table;
            const allCheckboxes = document.querySelectorAll(`.domain-checkbox[data-table="${table}"]`);
            const checkedCheckboxes = document.querySelectorAll(`.domain-checkbox[data-table="${table}"]:checked`);
            const selectAllCheckbox = document.querySelector(`.select-all-domains[data-table="${table}"]`);

            if (selectAllCheckbox) {
                selectAllCheckbox.checked = allCheckboxes.length === checkedCheckboxes.length && allCheckboxes.length > 0;
                selectAllCheckbox.indeterminate = checkedCheckboxes.length > 0 && checkedCheckboxes.length < allCheckboxes.length;
            }
        });
    });

    // Bulk action buttons
    const bulkClaimBtn = document.querySelector('.bulk-claim-btn');
    if (bulkClaimBtn) {
        bulkClaimBtn.addEventListener('click', bulkClaimDomains);
    }

    const bulkDeleteAllBtn = document.querySelector('.bulk-delete-all-btn');
    if (bulkDeleteAllBtn) {
        bulkDeleteAllBtn.addEventListener('click', () => bulkDeleteDomains('all'));
    }

    const bulkDeleteUnmanagedBtn = document.querySelector('.bulk-delete-unmanaged-btn');
    if (bulkDeleteUnmanagedBtn) {
        bulkDeleteUnmanagedBtn.addEventListener('click', () => bulkDeleteDomains('unmanaged'));
    }

    // Global event delegation for all buttons
    document.addEventListener('click', (e) => {
        if (e.target.closest('.revoke-session-btn')) {
            const btn = e.target.closest('.revoke-session-btn');
            const sessionId = btn.dataset.sessionId;
            revokeSession(sessionId);
        }

        // Delete user buttons (admin page)
        if (e.target.closest('.delete-user-btn')) {
            const btn = e.target.closest('.delete-user-btn');
            const userId = btn.dataset.userId;
            const email = btn.dataset.email;
            deleteUser(userId, email);
        }

        // Toggle user active buttons (admin page)
        if (e.target.closest('.toggle-user-btn')) {
            const btn = e.target.closest('.toggle-user-btn');
            const userId = btn.dataset.userId;
            const currentlyActive = btn.dataset.active === 'true';
            toggleUserActive(userId, currentlyActive);
        }

        // Admin delete domain buttons (admin page)
        if (e.target.closest('.admin-delete-domain-btn')) {
            const btn = e.target.closest('.admin-delete-domain-btn');
            const username = btn.dataset.username;
            const subdomain = btn.dataset.subdomain;
            adminDeleteDomain(username, subdomain);
        }

        // Show claim modal buttons (admin page)
        if (e.target.closest('.show-claim-modal-btn')) {
            const btn = e.target.closest('.show-claim-modal-btn');
            const username = btn.dataset.username;
            const subdomain = btn.dataset.subdomain;
            showClaimModal(username, subdomain);
        }
    });
});
