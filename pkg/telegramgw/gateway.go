// Package telegramgw provides a client for the Telegram Gateway API,
// which sends verification codes and messages via Telegram.
// See: https://core.telegram.org/gateway
package telegramgw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const defaultBaseURL = "https://gatewayapi.telegram.org"

// DeliveryStatus represents the status of a verification request.
type DeliveryStatus string

const (
	DeliveryStatusSent    DeliveryStatus = "sent"
	DeliveryStatusRead    DeliveryStatus = "read"
	DeliveryStatusRevoked DeliveryStatus = "revoked"
	DeliveryStatusExpired DeliveryStatus = "expired"
)

// SendCodeResult is returned by SendCode on success.
type SendCodeResult struct {
	RequestID   string `json:"request_id"`
	PhoneNumber string `json:"phone_number"`
}

// StatusResult is returned by CheckStatus.
type StatusResult struct {
	RequestID        string         `json:"request_id"`
	PhoneNumber      string         `json:"phone_number"`
	RequestCost      float64        `json:"request_cost"`
	RemainingBalance float64        `json:"remaining_balance"`
	DeliveryStatus   DeliveryStatus `json:"delivery_status"`
	IsCodeValid      *bool          `json:"is_code_valid,omitempty"`
	CodeEntered      string         `json:"code_entered,omitempty"`
}

// Gateway is the interface for Telegram Gateway operations.
// It is defined as an interface so that the real HTTP client and a test mock
// can both satisfy it.
type Gateway interface {
	// SendCode requests a verification code to be sent to the given phone number.
	// ttlSec is how long (seconds) the code should remain valid (30–3600).
	// Returns a RequestStatus containing the API-assigned request_id.
	SendCode(ctx context.Context, code, phone string, ttlSec int) (*SendCodeResult, error)

	// CheckStatus checks the delivery and verification status of a code request.
	CheckStatus(ctx context.Context, requestID string) (*StatusResult, error)

	// RevokeCode cancels an outstanding verification request.
	RevokeCode(ctx context.Context, requestID string) error
}

// HTTPGateway implements Gateway using the real Telegram Gateway HTTP API.
type HTTPGateway struct {
	apiToken string
	baseURL  string
	client   *http.Client
}

// NewHTTPGateway creates a new HTTP-backed Telegram Gateway client.
// If baseURL is empty the production URL is used.
func NewHTTPGateway(apiToken, baseURL string) *HTTPGateway {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &HTTPGateway{
		apiToken: apiToken,
		baseURL:  baseURL,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// --- API request/response types ---------------------------------------------

type sendCodeRequest struct {
	Code        string `json:"code"`
	PhoneNumber string `json:"phone_number"`
	TTL         int    `json:"ttl,omitempty"`
	CodeLength  int    `json:"code_length,omitempty"`
}

type checkStatusRequest struct {
	RequestID string `json:"request_id"`
}

type revokeCodeRequest struct {
	RequestID string `json:"request_id"`
}

type apiResponse[T any] struct {
	OK     bool   `json:"ok"`
	Result T      `json:"result"`
	Error  string `json:"error,omitempty"`
}

// --- Interface implementation -----------------------------------------------

func (g *HTTPGateway) SendCode(ctx context.Context, code, phone string, ttlSec int) (*SendCodeResult, error) {
	body := sendCodeRequest{
		Code:        code,
		PhoneNumber: phone,
		TTL:         ttlSec,
		CodeLength:  6,
	}

	var result SendCodeResult
	if err := g.post(ctx, "/sendVerificationMessage", body, &result); err != nil {
		return nil, fmt.Errorf("telegramgw: send code: %w", err)
	}
	return &result, nil
}

func (g *HTTPGateway) CheckStatus(ctx context.Context, requestID string) (*StatusResult, error) {
	body := checkStatusRequest{RequestID: requestID}

	var result StatusResult
	if err := g.post(ctx, "/checkVerificationStatus", body, &result); err != nil {
		return nil, fmt.Errorf("telegramgw: check status: %w", err)
	}
	return &result, nil
}

func (g *HTTPGateway) RevokeCode(ctx context.Context, requestID string) error {
	body := revokeCodeRequest{RequestID: requestID}
	var result any
	if err := g.post(ctx, "/revokeVerificationMessage", body, &result); err != nil {
		return fmt.Errorf("telegramgw: revoke code: %w", err)
	}
	return nil
}

// post is a helper that marshals body, sends a POST to path, and unmarshals the result.
func (g *HTTPGateway) post(ctx context.Context, path string, body, result any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.apiToken)

	resp, err := g.client.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var wrapper apiResponse[json.RawMessage]
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}
	if !wrapper.OK {
		return fmt.Errorf("api error: %s", wrapper.Error)
	}

	if result != nil && wrapper.Result != nil {
		if err := json.Unmarshal(wrapper.Result, result); err != nil {
			return fmt.Errorf("unmarshal result: %w", err)
		}
	}
	return nil
}
