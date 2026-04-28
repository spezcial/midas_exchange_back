# Frontend Implementation Guide — 2FA & Passkey

Base URL for all API calls: `/api/v1`

All protected endpoints require `Authorization: Bearer <access_token>`.

---

## 1. Login Flow

`POST /auth/login`

**Request**
```json
{ "email": "user@example.com", "password": "secret", "remember_me": false }
```

**Two possible responses:**

### A — No 2FA configured (user has no phone and no passkeys)
```json
{
  "status": "ok",
  "access_token": "...",
  "refresh_token": "...",
  "user": { ... },
  "two_factor_enabled": false,
  "passkey_enabled": false
}
```
Store tokens and redirect to the app.

### B — 2FA required
```json
{
  "status": "pending_2fa",
  "temp_token": "...",
  "methods": ["telegram"],
  "two_factor_enabled": true,
  "passkey_enabled": false
}
```
Save `temp_token` in memory (not localStorage — it's short-lived). Show the appropriate 2FA screen based on `methods`.

---

## 2. Complete Login — Telegram OTP

> Used when `methods` contains `"telegram"`.

No OTP is sent automatically. The frontend must first show a method picker, then trigger the send explicitly.

### Step 1 — request OTP (user chose Telegram)
`POST /auth/2fa/telegram/send`
```json
{ "temp_token": "<saved temp_token>" }
```
Success: `200 { "message": "Verification code sent" }` — show the 6-digit input.
429: rate-limited — show a retry timer.

### Step 2 — verify code
`POST /auth/2fa/telegram/verify`
```json
{ "temp_token": "<saved temp_token>", "code": "123456" }
```

**Success** → same shape as login status `"ok"` (`access_token`, `refresh_token`, `user`).

**Errors**
| Status | Meaning |
|--------|---------|
| 422 | Wrong code or max attempts reached — show inline error |
| 401 | Session expired — send user back to login |

---

## 3. Complete Login — Passkey (WebAuthn)

> Used when `methods` contains `"passkey"`.

WebAuthn requires two round trips. Use the browser's `navigator.credentials.get()` API.

### Step 1 — begin
`POST /auth/2fa/passkey/begin`
```json
{ "temp_token": "<saved temp_token>" }
```
Response:
```json
{ "session_id": "...", "assertion_options": { /* PublicKeyCredentialRequestOptions */ } }
```
Pass `assertion_options` to `navigator.credentials.get({ publicKey: assertionOptions })`.

### Step 2 — finish
`POST /auth/2fa/passkey/finish?session_id=<session_id>&temp_token=<temp_token>`

Body: the raw `PublicKeyCredential` JSON returned by the browser authenticator.

**Success** → same shape as login status `"ok"`.

---

## 4. Logout

`POST /auth/logout` *(protected)*
```json
{ "refresh_token": "..." }
```
Clear both tokens from storage on any response.

---

## 5. Forgot Password

Three-step flow, fully public (no auth required).

### Step 1 — request OTP
`POST /auth/forgot-password/send`
```json
{ "email": "user@example.com" }
```
Always returns 200 with a generic message. Show: *"If this email has a verified phone, you will receive a code."* Do not infer whether the account exists.

### Step 2 — verify OTP
`POST /auth/forgot-password/verify`
```json
{ "phone": "+77001234567", "code": "123456" }
```
Response on success:
```json
{ "reset_token": "..." }
```
On 422: invalid or expired code.

### Step 3 — set new password
`POST /auth/forgot-password/reset`
```json
{ "reset_token": "...", "new_password": "newSecret123" }
```
On success (200): redirect to login.
On 422: token expired — restart flow.

---

## 6. Phone Management (Settings / Profile)

All endpoints are protected.

### Add / update phone

**Step 1 — send OTP to new number**
`POST /profile/phone/send-otp`
```json
{ "phone": "+77001234567" }
```
Success: `200 { "message": "Verification code sent" }`
429: rate-limited — wait before retrying.
400: number already in use by another account.

**Step 2 — verify and save**
`POST /profile/phone/verify`
```json
{ "phone": "+77001234567", "code": "123456" }
```
Success: phone is now saved and `phone_verified = true`.
422: wrong code.

### Remove phone
`DELETE /profile/phone`

**Blocked** (409) if the user has no passkeys — phone is the only 2FA backup. Show an error: *"Register a passkey first before removing your phone."*

---

## 7. Passkey Management (Settings / Profile)

All endpoints are protected.

### List passkeys
`GET /profile/passkeys`

Response: array of credential objects
```json
[{ "id": 1, "name": "iPhone 15", "created_at": "..." }, ...]
```

### Register a new passkey

**Step 1 — begin**
`POST /profile/passkeys/register/begin`
```json
{ "name": "My MacBook" }
```
Response:
```json
{ "session_id": "...", "creation_options": { /* PublicKeyCredentialCreationOptions */ } }
```
Pass `creation_options` to `navigator.credentials.create({ publicKey: creationOptions })`.

**Step 2 — finish**
`POST /profile/passkeys/register/finish?session_id=<session_id>`

Body: the raw `PublicKeyCredential` JSON returned by the browser.

Response (201): the newly created credential object.

### Delete a passkey
`DELETE /profile/passkeys/{id}`

**Blocked** (409) if the user has no verified phone — passkey is the only 2FA method. Show: *"Add a verified phone number before removing this passkey."*

---

## 8. Action Token Flow (Withdraw / Change Password)

Sensitive actions require a pre-verified, single-use **action token** (5-min TTL). The flow is:

1. User clicks "Withdraw" (or "Change Password")
2. Frontend requests a challenge OTP **or** initiates a passkey assertion
3. User verifies → frontend receives `action_token`
4. Frontend includes `action_token` in the sensitive action request

### 8a — via Telegram OTP

**Request challenge**
`POST /auth/action-challenge` *(protected)*
```json
{ "action": "withdraw" }   // or "change_password"
```
Success: OTP sent to user's phone.
409: user has no verified phone — fall back to passkey flow.

**Verify and get token**
`POST /auth/action-verify/telegram` *(protected)*
```json
{ "action": "withdraw", "code": "123456" }
```
Success:
```json
{ "action_token": "..." }
```
422: wrong code or max attempts.

### 8b — via Passkey

**Begin assertion**
`POST /auth/action-verify/passkey/begin` *(protected)*
```json
{ "action": "withdraw" }
```
Response:
```json
{ "session_id": "...", "assertion_options": { /* PublicKeyCredentialRequestOptions */ } }
```

**Finish assertion**
`POST /auth/action-verify/passkey/finish` *(protected)*
```json
{ "action": "withdraw", "session_id": "..." }
```
Body also carries the raw `PublicKeyCredential` from the browser (merge into the JSON body).
Response:
```json
{ "action_token": "..." }
```

### Using the action token

Pass it as `"action_token"` in the body of the sensitive request:

**Withdraw example**
```json
POST /wallets/withdraw
{ "currency_id": 1, "amount": "100", "address": "0x...", "action_token": "..." }
```

**Change password example**
```json
PUT /auth/password
{ "current_password": "...", "new_password": "...", "action_token": "..." }
```

---

## 9. UI/UX Rules

- **If a user has neither phone nor passkeys**: login goes straight to tokens — no 2FA screen needed.
- **If both methods are available**: show a method-selector ("Use Telegram code" / "Use passkey") then proceed with the chosen flow.
- **Temp token is short-lived** (~5 min): show a countdown or let the user re-trigger login if the session expires.
- **Action token is single-use**: do not cache or reuse it. Fetch a fresh one for each action.
- **Phone must be verified** (`phone_verified: true`) before Telegram 2FA activates. A user who set their phone via an old profile-update UI (without OTP) must re-verify via `POST /profile/phone/send-otp` + `POST /profile/phone/verify`.
- **Passkey delete** is blocked without a verified phone backup, and vice-versa — at least one 2FA method must always remain.

---

## 10. Valid `action` Values

| Value | Used for |
|-------|----------|
| `withdraw` | Crypto/fiat withdrawal |
| `change_password` | Password change |
| `reset_password` | Forgot-password reset (internal — not used in action challenge flow) |

---

## 11. Error Shape

All errors follow:
```json
{ "error": "human-readable message" }
```

Common status codes:
| Code | Meaning |
|------|---------|
| 400 | Bad request / validation |
| 401 | Unauthenticated or session expired |
| 409 | Conflict (e.g. removing last 2FA method) |
| 422 | Wrong OTP code or expired token |
| 429 | OTP rate-limited — show retry timer |
| 500 | Server error |
