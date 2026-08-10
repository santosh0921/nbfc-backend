package surepass

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// ProviderError distinguishes "Surepass reached us and rejected the
// request" (bad token, rate limited, provider outage) from a genuine
// business-logic verification failure. Callers use errors.As to tell them
// apart — a ProviderError should never be presented to a customer as "your
// PAN is invalid"; that's a misconfiguration or outage on our end, not a
// finding about their document.
type ProviderError struct {
	StatusCode int
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("surepass: unexpected HTTP status %d", e.StatusCode)
}

type Client struct {
	BaseURL string
	Token   string
	Client  *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Post(endpoint string, body interface{}) ([]byte, error) {

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(
		http.MethodPost,
		c.BaseURL+endpoint,
		bytes.NewBuffer(jsonBody),
	)

	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Endpoint and status code only — never the response body. Surepass's
	// pan-comprehensive response carries the holder's full name, DOB, and
	// address alongside the PAN itself; internal/security/crypto.go exists
	// specifically so PAN is never stored in cleartext, and logging the raw
	// response here bypassed that entirely (it used to print the full body
	// via the println builtin, which is unbuffered/unstructured and can't
	// be suppressed by a log level).
	log.Printf("surepass: POST %s -> %s", endpoint, resp.Status)

	// Only 401/403/429/5xx indicate a problem with OUR request/account/the
	// provider itself (bad SUREPASS_BEARER_TOKEN, rate limited, provider
	// outage) — NOT a genuine business rejection. Surepass's
	// pan-comprehensive endpoint returns other 4xx codes (422, confirmed
	// live against the sandbox) for an ordinary "PAN not found" response
	// with a normal, parseable body — those must still flow through to the
	// caller to unmarshal and check Success on, exactly as a 200 would.
	// Conflating the two used to mean a misconfigured token or a provider
	// outage presented to every customer as "your PAN is invalid" and,
	// with no resubmission path, permanently stuck their KYC.
	if isProviderFailureStatus(resp.StatusCode) {
		return nil, &ProviderError{StatusCode: resp.StatusCode}
	}

	return bodyBytes, nil
}

func isProviderFailureStatus(code int) bool {
	return code == http.StatusUnauthorized ||
		code == http.StatusForbidden ||
		code == http.StatusTooManyRequests ||
		code >= 500
}