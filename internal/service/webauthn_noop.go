package service

import (
	"context"
	"errors"
	"net/http"

	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/go-webauthn/webauthn/protocol"
)

// noOpWebAuthnService is a WebAuthnService that rejects all requests.
// Used when WebAuthn is not configured.
type noOpWebAuthnService struct{}

// NewNoOpWebAuthnService returns a WebAuthnService that always returns errors.
// Used as a safe default when WebAuthn (RPID, etc.) is not configured.
func NewNoOpWebAuthnService() WebAuthnService {
	return &noOpWebAuthnService{}
}

var errWebAuthnNotConfigured = errors.New("passkey authentication is not configured on this server")

func (n *noOpWebAuthnService) BeginRegistration(_ context.Context, _ *domain.User, _ string) (*protocol.CredentialCreation, string, error) {
	return nil, "", errWebAuthnNotConfigured
}

func (n *noOpWebAuthnService) FinishRegistration(_ context.Context, _ int64, _ string, _ *http.Request) (*domain.PasskeyCredential, error) {
	return nil, errWebAuthnNotConfigured
}

func (n *noOpWebAuthnService) ListCredentials(_ context.Context, _ int64) ([]domain.PasskeyCredential, error) {
	return []domain.PasskeyCredential{}, nil
}

func (n *noOpWebAuthnService) DeleteCredential(_ context.Context, _ int64, _ int64) error {
	return errWebAuthnNotConfigured
}

func (n *noOpWebAuthnService) CountCredentials(_ context.Context, _ int64) (int, error) {
	return 0, nil
}

func (n *noOpWebAuthnService) BeginAuthentication(_ context.Context, _ int64) (*protocol.CredentialAssertion, string, error) {
	return nil, "", errWebAuthnNotConfigured
}

func (n *noOpWebAuthnService) FinishAuthentication(_ context.Context, _ int64, _ string, _ *http.Request) error {
	return errWebAuthnNotConfigured
}
