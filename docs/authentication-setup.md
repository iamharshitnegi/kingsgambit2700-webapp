# Password and Google sign-in

The full development stack supports username/email + password, Google sign-in,
email sign-in links, and student access codes. All methods use the same opaque,
HttpOnly session cookie. Coach and administrator roles must also complete TOTP
MFA before entering their workspace.

During authenticator setup, scan the displayed QR code with your authenticator
app and enter its six-digit code to confirm. The manual setup key and same-device
app link remain available. QR codes are generated locally in the browser from
the enrollment URI; setup secrets are not sent to a QR image service.

## Password accounts

At `/sign-in`, choose **Create an account**, enter a name, email, username and
12–128 character password, then open the emailed verification link. Local emails
are captured at <http://localhost:8025>. Registration creates a guardian account
only after mailbox verification; it cannot set a password on an existing account.
Usernames are case-insensitive and contain 3–32 letters, numbers, dots, hyphens or
underscores. Login accepts either the username or email. Passwords use individually
salted Argon2id hashes (19 MiB, two iterations, one lane).

Existing accounts without a password can still use **Email me a sign-in link**.
This is also the access fallback if a password is forgotten; a self-service
password reset/change flow is not included in this change.

For the seven previously provisioned local test accounts, usernames are
`guardian`, `student`, `coach`, `academics`, `operations`, `finance`, and `admin`.
Their emails are `<username>@example.test`. To assign or rotate their local
passwords after applying migrations:

```bash
(
  set -a
  . ./.env
  set +a
  read -rs -p 'Local test password (12–128 characters): ' KG_LOCAL_TEST_PASSWORD
  export KG_LOCAL_TEST_PASSWORD
  cd backend
  go run ./cmd/local-passwords
)
```

The utility requires `KG_ENV=development` and the `kg2700` database on a loopback
host. It only changes those seven test accounts, audits each change, and revokes
their old sessions. It does not change roles or authenticator enrollment.

## Enable Google sign-in

1. Open [Google Auth Platform](https://console.cloud.google.com/auth/overview)
   in your Google Cloud project. Configure the application's branding, audience,
   and test users if the consent screen is in testing mode.
2. Create an OAuth client with application type **Web application**.
3. Add `http://localhost` and `http://localhost:3000` to **Authorized JavaScript origins**. Add each exact
   production frontend origin separately. This implementation uses Google's
   JavaScript popup credential callback, so no redirect URI or client secret is
   used by the application.
4. Set the client ID in the root `.env`:

   ```dotenv
   KG_GOOGLE_CLIENT_ID=YOUR_CLIENT_ID.apps.googleusercontent.com
   ```

5. Restart the API (`make backend-api`, or recreate the Compose API service),
   then refresh `/sign-in`. **Continue with Google** appears on both the login
   and registration forms. No frontend rebuild is needed for the client ID.

Google sign-in creates a guardian account on first use. Returning accounts are
matched by Google's immutable subject. An existing active account can be linked
by email only when Google is authoritative for that verified Gmail/Workspace
address. Google accounts using a third-party email must register through mailbox
verification or use their existing password/email-link sign-in method.
Google sign-in never grants staff roles or bypasses staff MFA.

The backend validates Google's signature, audience, issuer, expiry, verified
email, and browser-bound one-time nonce. It never accepts a client-supplied user
ID or email as proof of Google authentication. Without the client ID, Google
sign-in is disabled and its button is hidden; password sign-in remains available.

Reference: [Google Identity Services setup](https://developers.google.com/identity/gsi/web/guides/get-google-api-clientid)
and [server-side token verification](https://developers.google.com/identity/gsi/web/guides/verify-google-id-token).

The browser-only demo retains its fictional account picker and email continuation;
it does not use real passwords or Google authentication.
