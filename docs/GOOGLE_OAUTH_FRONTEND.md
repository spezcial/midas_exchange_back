# Google OAuth Frontend Integration Guide

This document describes how to integrate Google OAuth into your frontend application with the CaspianEx backend.

## Overview

The OAuth flow uses a two-step process:
1. Frontend requests OAuth URL from backend
2. After Google authentication, frontend sends the authorization code to backend

## API Endpoints

### 1. Get Google OAuth URL

```
GET /api/v1/auth/google/url
```

**Response:**
```json
{
  "success": true,
  "data": {
    "auth_url": "https://accounts.google.com/o/oauth2/v2/auth?...",
    "state": "random-state-string"
  }
}
```

### 2. Exchange Authorization Code

```
POST /api/v1/auth/google/callback
Content-Type: application/json

{
  "code": "authorization-code-from-google",
  "state": "state-from-step-1"
}
```

**Success Response:**
```json
{
  "success": true,
  "data": {
    "access_token": "jwt-access-token",
    "refresh_token": "refresh-token",
    "user": {
      "id": 1,
      "email": "user@gmail.com",
      "first_name": "John",
      "last_name": "Doe",
      "role": "client",
      "is_active": true,
      "is_verified": true,
      "auth_method": "google",
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-01T00:00:00Z"
    }
  }
}
```

**Error Response:**
```json
{
  "success": false,
  "error": "email already registered. Please login with password"
}
```

## Implementation Example (React/TypeScript)

### Step 1: Create Google Auth Button Component

```typescript
// components/GoogleAuthButton.tsx
import { useState } from 'react';
import { api } from '../services/api';

export function GoogleAuthButton() {
  const [loading, setLoading] = useState(false);

  const handleGoogleLogin = async () => {
    try {
      setLoading(true);

      // Get OAuth URL from backend
      const response = await api.get('/auth/google/url');
      const { auth_url, state } = response.data.data;

      // Store state for verification
      sessionStorage.setItem('oauth_state', state);

      // Redirect to Google
      window.location.href = auth_url;
    } catch (error) {
      console.error('Failed to initiate Google login:', error);
      setLoading(false);
    }
  };

  return (
    <button onClick={handleGoogleLogin} disabled={loading}>
      {loading ? 'Loading...' : 'Continue with Google'}
    </button>
  );
}
```

### Step 2: Create OAuth Callback Page

```typescript
// pages/auth/GoogleCallback.tsx
import { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { api } from '../../services/api';
import { useAuth } from '../../contexts/AuthContext';

export function GoogleCallback() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { setUser, setTokens } = useAuth();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const handleCallback = async () => {
      const code = searchParams.get('code');
      const state = searchParams.get('state');
      const storedState = sessionStorage.getItem('oauth_state');

      // Clear stored state
      sessionStorage.removeItem('oauth_state');

      // Validate state
      if (!state || state !== storedState) {
        setError('Invalid state. Please try again.');
        return;
      }

      if (!code) {
        setError('No authorization code received.');
        return;
      }

      try {
        // Exchange code for tokens
        const response = await api.post('/auth/google/callback', {
          code,
          state,
        });

        const { access_token, refresh_token, user } = response.data.data;

        // Store tokens and user
        setTokens(access_token, refresh_token);
        setUser(user);

        // Redirect to dashboard
        navigate('/dashboard');
      } catch (error: any) {
        const message = error.response?.data?.error || 'Authentication failed';
        setError(message);
      }
    };

    handleCallback();
  }, [searchParams, navigate, setUser, setTokens]);

  if (error) {
    return (
      <div>
        <h2>Authentication Error</h2>
        <p>{error}</p>
        <a href="/login">Back to Login</a>
      </div>
    );
  }

  return (
    <div>
      <p>Authenticating...</p>
    </div>
  );
}
```

### Step 3: Add Route for Callback

```typescript
// App.tsx or routes.tsx
import { GoogleCallback } from './pages/auth/GoogleCallback';

// Add this route (must match GOOGLE_OAUTH_REDIRECT_URI)
<Route path="/auth/google/callback" element={<GoogleCallback />} />
```

## Configuration

### Backend Environment Variables

```bash
GOOGLE_OAUTH_CLIENT_ID=your-google-client-id
GOOGLE_OAUTH_CLIENT_SECRET=your-google-client-secret
GOOGLE_OAUTH_REDIRECT_URI=http://localhost:5173/auth/google/callback
```

### Google Cloud Console Setup

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select existing
3. Enable Google+ API and Google OAuth2 API
4. Go to "Credentials" > "Create Credentials" > "OAuth 2.0 Client ID"
5. Select "Web application"
6. Add Authorized redirect URIs:
   - Development: `http://localhost:5173/auth/google/callback`
   - Production: `https://yourdomain.com/auth/google/callback`
7. Copy Client ID and Client Secret to your `.env` file

## Flow Diagram

```
┌──────────┐     1. Click "Login with Google"      ┌─────────┐
│ Frontend │ ────────────────────────────────────> │ Backend │
│          │ <──────────────────────────────────── │         │
│          │     2. Return {auth_url, state}       │         │
└────┬─────┘                                       └─────────┘
     │
     │ 3. Redirect to Google
     v
┌──────────┐
│  Google  │
│  OAuth   │
└────┬─────┘
     │
     │ 4. User authenticates
     │ 5. Redirect to frontend callback with code & state
     v
┌──────────┐     6. POST {code, state}             ┌─────────┐
│ Frontend │ ────────────────────────────────────> │ Backend │
│ Callback │                                       │         │
│          │                                       │    │    │
│          │                                       │    v    │
│          │                                       │ Exchange│
│          │                                       │ code    │
│          │                                       │ with    │
│          │                                       │ Google  │
│          │ <──────────────────────────────────── │         │
│          │     7. Return {access_token,          │         │
│          │        refresh_token, user}           │         │
└──────────┘                                       └─────────┘
```

## Error Handling

| Error Message | Description | User Action |
|--------------|-------------|-------------|
| "Invalid or expired state" | CSRF protection triggered | Retry login |
| "Failed to exchange authorization code" | Google rejected the code | Retry login |
| "Google email is not verified" | User's Google email not verified | Verify email in Google |
| "Email already registered. Please login with password" | Email exists with password auth | Use password login |

## Security Notes

1. **State Parameter**: Always validate the state parameter to prevent CSRF attacks
2. **HTTPS**: Use HTTPS in production for the redirect URI
3. **Token Storage**: Store tokens securely (httpOnly cookies recommended)
4. **State Expiry**: OAuth states expire after 10 minutes
