package domain

import "time"

// PasskeyCredential represents a WebAuthn / FIDO2 credential registered by a user.
type PasskeyCredential struct {
	ID           int64      `db:"id"            json:"id"`
	UserID       int64      `db:"user_id"       json:"-"`
	CredentialID string     `db:"credential_id" json:"credential_id"` // base64url
	PublicKey    []byte     `db:"public_key"    json:"-"`
	AAGUID       []byte     `db:"aaguid"        json:"-"`
	SignCount      uint32     `db:"sign_count"       json:"-"`
	BackupEligible bool       `db:"backup_eligible"  json:"-"`
	Name           string     `db:"name"             json:"name"`
	CreatedAt    time.Time  `db:"created_at"    json:"created_at"`
	LastUsedAt   *time.Time `db:"last_used_at"  json:"last_used_at,omitempty"`
}

// LoginFingerprint is a record of a known client (IP + UA hash) for a user.
type LoginFingerprint struct {
	ID          int64     `db:"id"`
	UserID      int64     `db:"user_id"`
	Fingerprint string    `db:"fingerprint"`
	CreatedAt   time.Time `db:"created_at"`
	LastSeenAt  time.Time `db:"last_seen_at"`
}
