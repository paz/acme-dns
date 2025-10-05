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

// Dashboard functions
function viewCredentials(username) {
    fetch(`/dashboard/domain/${username}/credentials`, {
        headers: {
            'X-CSRF-Token': csrfToken
        }
    })
    .then(response => response.json())
    .then(data => {
        document.getElementById('cred-username').value = data.username;
        document.getElementById('cred-password').value = data.password;
        document.getElementById('cred-fulldomain').value = data.fulldomain;

        // Generate curl example
        const curlCmd = `curl -X POST https://auth.example.org/update \\
  -H "X-Api-User: ${data.username}" \\
  -H "X-Api-Key: ${data.password}" \\
  -d '{"subdomain": "${data.subdomain}", "txt": "___validation_token_received_from_the_ca___"}'`;
        document.getElementById('curl-example').textContent = curlCmd;

        const modal = new bootstrap.Modal(document.getElementById('credentialsModal'));
        modal.show();
    })
    .catch(error => {
        console.error('Error:', error);
        showToast('Failed to load credentials', 'danger');
    });
}

function deleteDomain(username, subdomain) {
    if (!confirm(`Are you sure you want to delete ${subdomain}?`)) {
        return;
    }

    fetch(`/dashboard/domain/${username}`, {
        method: 'DELETE',
        headers: {
            'Content-Type': 'application/json',
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
