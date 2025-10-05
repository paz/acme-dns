package email

import (
	"fmt"
	"html/template"
	"strings"
)

// PasswordResetEmail generates a password reset email
func PasswordResetEmail(email, resetToken, resetURL string) (subject, body string) {
	subject = "Password Reset Request - acme-dns"

	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
        }
        .header {
            background: #0d6efd;
            color: white;
            padding: 20px;
            text-align: center;
            border-radius: 5px 5px 0 0;
        }
        .content {
            background: #f8f9fa;
            padding: 30px;
            border-radius: 0 0 5px 5px;
        }
        .button {
            display: inline-block;
            background: #0d6efd;
            color: white !important;
            padding: 12px 30px;
            text-decoration: none;
            border-radius: 5px;
            margin: 20px 0;
        }
        .footer {
            margin-top: 30px;
            font-size: 12px;
            color: #666;
            text-align: center;
        }
        code {
            background: #e9ecef;
            padding: 2px 6px;
            border-radius: 3px;
            font-family: 'Courier New', monospace;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>🔐 Password Reset Request</h1>
    </div>
    <div class="content">
        <p>Hello,</p>
        <p>We received a request to reset the password for your acme-dns account (<strong>{{.Email}}</strong>).</p>
        <p>Click the button below to reset your password:</p>
        <p style="text-align: center;">
            <a href="{{.ResetURL}}" class="button">Reset Password</a>
        </p>
        <p>Or copy and paste this link into your browser:</p>
        <p><code>{{.ResetURL}}</code></p>
        <p><strong>This link will expire in 1 hour.</strong></p>
        <p>If you didn't request this password reset, you can safely ignore this email. Your password will remain unchanged.</p>
    </div>
    <div class="footer">
        <p>This is an automated message from acme-dns. Please do not reply to this email.</p>
    </div>
</body>
</html>
`

	data := struct {
		Email    string
		ResetURL string
	}{
		Email:    template.HTMLEscapeString(email),
		ResetURL: template.HTMLEscapeString(resetURL),
	}

	t, err := template.New("password_reset").Parse(tmpl)
	if err != nil {
		// Fallback to simple template
		body = fmt.Sprintf("Password reset requested for %s. Reset URL: %s", email, resetURL)
		return
	}

	var buf strings.Builder
	err = t.Execute(&buf, data)
	if err != nil {
		body = fmt.Sprintf("Password reset requested for %s. Reset URL: %s", email, resetURL)
		return
	}

	body = buf.String()
	return
}

// WelcomeEmail generates a welcome email for new users
func WelcomeEmail(email, tempPassword string) (subject, body string) {
	subject = "Welcome to acme-dns!"

	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
        }
        .header {
            background: #198754;
            color: white;
            padding: 20px;
            text-align: center;
            border-radius: 5px 5px 0 0;
        }
        .content {
            background: #f8f9fa;
            padding: 30px;
            border-radius: 0 0 5px 5px;
        }
        .credentials {
            background: white;
            border: 1px solid #dee2e6;
            border-radius: 5px;
            padding: 15px;
            margin: 20px 0;
        }
        .footer {
            margin-top: 30px;
            font-size: 12px;
            color: #666;
            text-align: center;
        }
        code {
            background: #e9ecef;
            padding: 2px 6px;
            border-radius: 3px;
            font-family: 'Courier New', monospace;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>👋 Welcome to acme-dns!</h1>
    </div>
    <div class="content">
        <p>Hello,</p>
        <p>Your acme-dns account has been created by an administrator.</p>
        <div class="credentials">
            <p><strong>Email:</strong> <code>{{.Email}}</code></p>
            <p><strong>Temporary Password:</strong> <code>{{.TempPassword}}</code></p>
        </div>
        <p><strong>⚠️ Important:</strong> Please log in and change your password immediately.</p>
        <p>You can log in at your acme-dns instance and manage your DNS challenge records through the web interface.</p>
    </div>
    <div class="footer">
        <p>This is an automated message from acme-dns. Please do not reply to this email.</p>
    </div>
</body>
</html>
`

	data := struct {
		Email        string
		TempPassword string
	}{
		Email:        template.HTMLEscapeString(email),
		TempPassword: template.HTMLEscapeString(tempPassword),
	}

	t, err := template.New("welcome").Parse(tmpl)
	if err != nil {
		body = fmt.Sprintf("Welcome! Your temporary password is: %s", tempPassword)
		return
	}

	var buf strings.Builder
	err = t.Execute(&buf, data)
	if err != nil {
		body = fmt.Sprintf("Welcome! Your temporary password is: %s", tempPassword)
		return
	}

	body = buf.String()
	return
}

// TestEmail generates a test email to verify SMTP configuration
func TestEmail(toEmail string) (subject, body string) {
	subject = "acme-dns - Email Configuration Test"

	body = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
        }
        .header {
            background: #0dcaf0;
            color: #000;
            padding: 20px;
            text-align: center;
            border-radius: 5px 5px 0 0;
        }
        .content {
            background: #f8f9fa;
            padding: 30px;
            border-radius: 0 0 5px 5px;
        }
        .footer {
            margin-top: 30px;
            font-size: 12px;
            color: #666;
            text-align: center;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>✅ Email Test Successful!</h1>
    </div>
    <div class="content">
        <p>Congratulations!</p>
        <p>Your acme-dns email configuration is working correctly.</p>
        <p>You are now able to send password reset emails and other notifications.</p>
    </div>
    <div class="footer">
        <p>This is an automated test message from acme-dns.</p>
    </div>
</body>
</html>
`

	return
}
