package webhook

import (
	"net/http/httptest"
	"testing"
)

// newTestHandler builds a CryptoGateHandler with only the fields verifySecret
// needs. cgService and log are intentionally nil — verifySecret must not touch
// either, and a panic would surface as a test failure.
func newTestHandler(secret string) *CryptoGateHandler {
	return &CryptoGateHandler{
		webhookSecret: secret,
	}
}

func TestVerifySecret(t *testing.T) {
	tests := []struct {
		name        string
		configured  string // value of h.webhookSecret
		headerToken string // value sent in X-TOKEN header
		setHeader   bool   // whether to set the X-TOKEN header at all
		want        bool
	}{
		{
			name:        "empty configured secret rejects matching empty token (fail-closed)",
			configured:  "",
			headerToken: "",
			setHeader:   true,
			want:        false,
		},
		{
			name:       "empty configured secret rejects when no header is sent (fail-closed)",
			configured: "",
			setHeader:  false,
			want:       false,
		},
		{
			name:        "matching secret is accepted",
			configured:  "super-secret-token",
			headerToken: "super-secret-token",
			setHeader:   true,
			want:        true,
		},
		{
			name:        "mismatched secret of equal length is rejected",
			configured:  "super-secret-tokeX",
			headerToken: "super-secret-token",
			setHeader:   true,
			want:        false,
		},
		{
			name:        "shorter token vs longer secret is rejected without panic",
			configured:  "longvalue",
			headerToken: "shorter",
			setHeader:   true,
			want:        false,
		},
		{
			name:        "longer token vs shorter secret is rejected without panic",
			configured:  "shorter",
			headerToken: "longvalue",
			setHeader:   true,
			want:        false,
		},
		{
			name:       "missing X-TOKEN header is rejected",
			configured: "super-secret-token",
			setHeader:  false,
			want:       false,
		},
		{
			name:        "empty token against configured secret is rejected",
			configured:  "super-secret-token",
			headerToken: "",
			setHeader:   true,
			want:        false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandler(tc.configured)
			req := httptest.NewRequest("POST", "/cg/deposit", nil)
			if tc.setHeader {
				req.Header.Set("X-TOKEN", tc.headerToken)
			}

			got := h.verifySecret(req)
			if got != tc.want {
				t.Fatalf("verifySecret() = %v, want %v (configured=%q, headerToken=%q, setHeader=%v)",
					got, tc.want, tc.configured, tc.headerToken, tc.setHeader)
			}
		})
	}
}

// TestVerifySecret_DifferentLengthsNoPanic explicitly asserts the constant-time
// comparator does not panic when the supplied X-TOKEN and the configured secret
// have different byte lengths. subtle.ConstantTimeCompare returns 0 in that
// case, but the safety guarantee is worth pinning down with a dedicated test
// that would fail loudly via a recover() if the implementation regressed.
func TestVerifySecret_DifferentLengthsNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("verifySecret panicked on different-length inputs: %v", r)
		}
	}()

	h := newTestHandler("longvalue")
	req := httptest.NewRequest("POST", "/cg/deposit", nil)
	req.Header.Set("X-TOKEN", "shorter")

	if got := h.verifySecret(req); got {
		t.Fatalf("verifySecret() = true for different-length inputs, want false")
	}
}
