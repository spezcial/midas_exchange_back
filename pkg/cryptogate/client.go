// Package cryptogate is a thin HTTP client for the crypto-gate payment gateway.
// It calls two endpoints: GET /wallet (deposit address creation) and POST /withdraw.
package cryptogate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client communicates with a running crypto-gate instance.
type Client struct {
	baseURL    string
	token      string
	platform   string // platform slug assigned during gateway registration
	httpClient *http.Client
}

func NewClient(baseURL, token, platform string) *Client {
	return &Client{
		baseURL:  baseURL,
		token:    token,
		platform: platform,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Ping checks that the gateway is reachable and the token is valid.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/ping", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-TOKEN", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("cryptogate: ping: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cryptogate: ping: status %d: %s", resp.StatusCode, body)
	}
	return nil
}

// GetDepositAddress requests a new deposit address for a chain from the gateway.
// chain must be one of: bitcoin | ethereum | binance | tron
func (c *Client) GetDepositAddress(ctx context.Context, chain string) (string, error) {
	u := fmt.Sprintf("%s/wallet?chain=%s&platform=%s", c.baseURL,
		url.QueryEscape(chain), url.QueryEscape(c.platform))

	fmt.Println("here", c.token, c.platform)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-TOKEN", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("cryptogate: get deposit address: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cryptogate: get deposit address: status %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("cryptogate: get deposit address: decode: %w", err)
	}
	if result.Address == "" {
		return "", fmt.Errorf("cryptogate: get deposit address: empty address in response")
	}
	return result.Address, nil
}

// GatewayAddress is one entry from the GET /wallet/addresses response.
type GatewayAddress struct {
	Address   string `json:"address"`
	Chain     string `json:"chain"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// ListPlatformAddresses fetches every address the gateway has registered for this platform.
// Use it to audit or reconcile your local address table against the gateway.
func (c *Client) ListPlatformAddresses(ctx context.Context) ([]GatewayAddress, error) {
	u := fmt.Sprintf("%s/wallet/addresses?platform=%s", c.baseURL, url.QueryEscape(c.platform))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-TOKEN", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cryptogate: list addresses: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cryptogate: list addresses: status %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Addresses []GatewayAddress `json:"addresses"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("cryptogate: list addresses: decode: %w", err)
	}
	return result.Addresses, nil
}

// WithdrawRequest is the payload sent to POST /withdraw.
type WithdrawRequest struct {
	UUID     string `json:"uuid"`     // exchange-side transaction ID (string)
	Address  string `json:"address"`  // recipient blockchain address
	Amount   string `json:"amount"`   // decimal string e.g. "1.5"
	Asset    string `json:"asset"`    // btc | eth | bnb | trx | ethereum_<contract> | …
	Platform string `json:"platform"` // platform slug — required by the gateway
}

// Withdraw submits a withdrawal to crypto-gate.
// On success returns the transaction hash returned by the gateway.
func (c *Client) Withdraw(ctx context.Context, req WithdrawRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/withdraw", c.baseURL), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-TOKEN", c.token)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("cryptogate: withdraw: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Success string `json:"success"` // tx hash on success
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("cryptogate: withdraw: decode: %w", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("cryptogate: withdraw: %s", result.Error)
	}
	return result.Success, nil
}
