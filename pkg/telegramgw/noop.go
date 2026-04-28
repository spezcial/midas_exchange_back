package telegramgw

import "context"

// NoOpGateway is a Gateway implementation that does nothing.
// Used when the Telegram Gateway API token is not configured (e.g. local dev).
type NoOpGateway struct{}

func (n *NoOpGateway) SendCode(_ context.Context, _, _ string, _ int) (*SendCodeResult, error) {
	return &SendCodeResult{}, nil
}

func (n *NoOpGateway) CheckStatus(_ context.Context, _ string) (*StatusResult, error) {
	return &StatusResult{}, nil
}

func (n *NoOpGateway) RevokeCode(_ context.Context, _ string) error { return nil }
