package service

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/internal/repository"
	"github.com/caspianex/exchange-backend/pkg/cache"
	"github.com/caspianex/exchange-backend/pkg/logger"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

const passkeySessionTTL = 5 * time.Minute

// WebAuthnService manages passkey (FIDO2 / WebAuthn) registration and authentication.
type WebAuthnService interface {
	// BeginRegistration starts the credential creation ceremony for a user.
	// Returns the JSON-serialisable creation options and a sessionID to pass
	// back when calling FinishRegistration.
	BeginRegistration(ctx context.Context, user *domain.User, credName string) (creation *protocol.CredentialCreation, sessionID string, err error)

	// FinishRegistration validates the authenticator's attestation and persists
	// the new credential.
	FinishRegistration(ctx context.Context, userID int64, sessionID string, r *http.Request) (*domain.PasskeyCredential, error)

	// ListCredentials returns all passkeys registered by a user.
	ListCredentials(ctx context.Context, userID int64) ([]domain.PasskeyCredential, error)

	// DeleteCredential removes a specific passkey.  The caller is responsible for
	// checking that the deletion is allowed (i.e. user still has a phone set).
	DeleteCredential(ctx context.Context, id int64, userID int64) error

	// CountCredentials returns the number of passkeys the user has.
	CountCredentials(ctx context.Context, userID int64) (int, error)

	// BeginAuthentication starts the assertion ceremony.
	// Returns assertion options and a sessionID used by FinishAuthentication.
	BeginAuthentication(ctx context.Context, userID int64) (assertion *protocol.CredentialAssertion, sessionID string, err error)

	// FinishAuthentication validates the authenticator's assertion.
	// The credential's sign_count is updated on success to prevent replay/clone attacks.
	FinishAuthentication(ctx context.Context, userID int64, sessionID string, r *http.Request) error
}

// webAuthnUser is an adapter that satisfies the webauthn.User interface without
// importing the webauthn library directly into domain.
type webAuthnUser struct {
	user  *domain.User
	creds []domain.PasskeyCredential
}

func (u *webAuthnUser) WebAuthnID() []byte {
	// Use an 8-byte big-endian encoding of the numeric user ID.
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(u.user.ID))
	return b
}

func (u *webAuthnUser) WebAuthnName() string        { return u.user.Email }
func (u *webAuthnUser) WebAuthnDisplayName() string { return u.user.FirstName + " " + u.user.LastName }

func (u *webAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	out := make([]webauthn.Credential, 0, len(u.creds))
	for _, c := range u.creds {
		rawID, err := base64.RawURLEncoding.DecodeString(c.CredentialID)
		if err != nil {
			continue
		}
		out = append(out, webauthn.Credential{
			ID:        rawID,
			PublicKey: c.PublicKey,
			Flags: webauthn.CredentialFlags{
				BackupEligible: c.BackupEligible,
				BackupState:    c.BackupState,
			},
			Authenticator: webauthn.Authenticator{
				AAGUID:    c.AAGUID,
				SignCount: c.SignCount,
			},
		})
	}
	return out
}

// webAuthnSessionPayload is what we store in Redis for an in-progress WebAuthn ceremony.
type webAuthnSessionPayload struct {
	SessionData webauthn.SessionData `json:"session_data"`
	CredName    string               `json:"cred_name,omitempty"` // set only during registration
}

type webAuthnService struct {
	wa          *webauthn.WebAuthn
	passkeyRepo repository.PasskeyRepository
	store       cache.Store
	logger      *logger.Logger
}

// NewWebAuthnService creates a WebAuthnService.
// rpID should be the bare domain (e.g. "caspianex.com").
// rpOrigins should include the full origins (e.g. ["https://caspianex.com", "http://localhost:5173"]).
func NewWebAuthnService(
	rpID, rpDisplayName string,
	rpOrigins []string,
	passkeyRepo repository.PasskeyRepository,
	store cache.Store,
	log *logger.Logger,
) (WebAuthnService, error) {
	wa, err := webauthn.New(&webauthn.Config{
		RPID:          rpID,
		RPDisplayName: rpDisplayName,
		RPOrigins:     rpOrigins,
	})
	if err != nil {
		return nil, fmt.Errorf("webauthn init: %w", err)
	}
	return &webAuthnService{
		wa:          wa,
		passkeyRepo: passkeyRepo,
		store:       store,
		logger:      log,
	}, nil
}

// --- Registration ceremony --------------------------------------------------

func (s *webAuthnService) BeginRegistration(ctx context.Context, user *domain.User, credName string) (*protocol.CredentialCreation, string, error) {
	existing, err := s.passkeyRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		return nil, "", fmt.Errorf("webauthn: load credentials: %w", err)
	}

	wUser := &webAuthnUser{user: user, creds: existing}

	// Exclude already-registered credentials to prevent re-registration of same device.
	excludeList := make([]protocol.CredentialDescriptor, 0, len(existing))
	for _, c := range existing {
		rawID, _ := base64.RawURLEncoding.DecodeString(c.CredentialID)
		excludeList = append(excludeList, protocol.CredentialDescriptor{
			Type:         protocol.PublicKeyCredentialType,
			CredentialID: rawID,
		})
	}

	creation, sessionData, err := s.wa.BeginRegistration(wUser,
		webauthn.WithExclusions(excludeList),
	)
	if err != nil {
		return nil, "", fmt.Errorf("webauthn: begin registration: %w", err)
	}

	sessionID, err := s.storeSession(ctx, webAuthnSessionPayload{
		SessionData: *sessionData,
		CredName:    credName,
	})
	if err != nil {
		return nil, "", err
	}

	return creation, sessionID, nil
}

func (s *webAuthnService) FinishRegistration(ctx context.Context, userID int64, sessionID string, r *http.Request) (*domain.PasskeyCredential, error) {
	payload, err := s.consumeSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// We need a minimal webAuthnUser with credentials loaded (for exclusion check).
	existingCreds, _ := s.passkeyRepo.GetByUserID(ctx, userID)
	dummyUser := &domain.User{ID: userID}
	wUser := &webAuthnUser{user: dummyUser, creds: existingCreds}

	cred, err := s.wa.FinishRegistration(wUser, payload.SessionData, r)
	if err != nil {
		return nil, fmt.Errorf("webauthn: finish registration: %w", err)
	}

	name := payload.CredName
	if name == "" {
		name = "Passkey"
	}

	domainCred := &domain.PasskeyCredential{
		UserID:         userID,
		CredentialID:   base64.RawURLEncoding.EncodeToString(cred.ID),
		PublicKey:      cred.PublicKey,
		AAGUID:         cred.Authenticator.AAGUID,
		SignCount:      cred.Authenticator.SignCount,
		BackupEligible: cred.Flags.BackupEligible,
		BackupState:    cred.Flags.BackupState,
		Name:           name,
	}

	if err := s.passkeyRepo.Create(ctx, domainCred); err != nil {
		return nil, fmt.Errorf("webauthn: persist credential: %w", err)
	}

	return domainCred, nil
}

// --- Authentication ceremony ------------------------------------------------

func (s *webAuthnService) BeginAuthentication(ctx context.Context, userID int64) (*protocol.CredentialAssertion, string, error) {
	existingCreds, err := s.passkeyRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, "", fmt.Errorf("webauthn: load credentials: %w", err)
	}
	if len(existingCreds) == 0 {
		return nil, "", errors.New("no passkeys registered for this account")
	}

	dummyUser := &domain.User{ID: userID}
	wUser := &webAuthnUser{user: dummyUser, creds: existingCreds}

	assertion, sessionData, err := s.wa.BeginLogin(wUser)
	if err != nil {
		return nil, "", fmt.Errorf("webauthn: begin login: %w", err)
	}

	sessionID, err := s.storeSession(ctx, webAuthnSessionPayload{SessionData: *sessionData})
	if err != nil {
		return nil, "", err
	}

	return assertion, sessionID, nil
}

func (s *webAuthnService) FinishAuthentication(ctx context.Context, userID int64, sessionID string, r *http.Request) error {
	payload, err := s.consumeSession(ctx, sessionID)
	if err != nil {
		return err
	}

	existingCreds, err := s.passkeyRepo.GetByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("webauthn: load credentials: %w", err)
	}

	dummyUser := &domain.User{ID: userID}
	wUser := &webAuthnUser{user: dummyUser, creds: existingCreds}

	updatedCred, err := s.wa.FinishLogin(wUser, payload.SessionData, r)
	if err != nil {
		s.logger.Error("webauthn: finish login failed", "user_id", userID, "error", err)
		return fmt.Errorf("webauthn: finish login: %w", err)
	}

	// Update sign_count in DB to detect cloned credentials on future attempts.
	credID := base64.RawURLEncoding.EncodeToString(updatedCred.ID)
	if err := s.passkeyRepo.UpdateSignCount(ctx, credID, updatedCred.Authenticator.SignCount, updatedCred.Flags.BackupState); err != nil {
		// Non-fatal — don't fail the login, but log so ops can detect persistent drift.
		s.logger.Warn("failed to update passkey sign_count — clone detection may be impaired",
			"cred_id", credID, "error", err)
	}

	return nil
}

// --- Helpers -----------------------------------------------------------------

func (s *webAuthnService) ListCredentials(ctx context.Context, userID int64) ([]domain.PasskeyCredential, error) {
	return s.passkeyRepo.GetByUserID(ctx, userID)
}

func (s *webAuthnService) DeleteCredential(ctx context.Context, id int64, userID int64) error {
	return s.passkeyRepo.Delete(ctx, id, userID)
}

func (s *webAuthnService) CountCredentials(ctx context.Context, userID int64) (int, error) {
	return s.passkeyRepo.CountByUserID(ctx, userID)
}

func (s *webAuthnService) storeSession(ctx context.Context, payload webAuthnSessionPayload) (string, error) {
	sessionID, err := generateSecureToken()
	if err != nil {
		return "", fmt.Errorf("webauthn: generate session id: %w", err)
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("webauthn: marshal session: %w", err)
	}

	key := passkeySessionKey(sessionID)
	if err := s.store.Set(ctx, key, b, passkeySessionTTL); err != nil {
		return "", fmt.Errorf("webauthn: store session: %w", err)
	}
	return sessionID, nil
}

func (s *webAuthnService) consumeSession(ctx context.Context, sessionID string) (*webAuthnSessionPayload, error) {
	raw, found, err := s.store.GetDel(ctx, passkeySessionKey(sessionID))
	if err != nil {
		return nil, fmt.Errorf("webauthn: read session: %w", err)
	}
	if !found {
		return nil, errors.New("passkey session expired or not found — please restart the ceremony")
	}

	var p webAuthnSessionPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("webauthn: unmarshal session: %w", err)
	}
	return &p, nil
}

func passkeySessionKey(sessionID string) string {
	return fmt.Sprintf("passkey_session:%s", sessionID)
}

