package gate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// SignalClient sends notifications via the Signal REST API.
type SignalClient struct {
	httpClient *http.Client
	apiURL     string
	recipient  string
}

func NewSignalClient(apiURL, recipient string) *SignalClient {
	return &SignalClient{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		apiURL:     apiURL,
		recipient:  recipient,
	}
}

func (c *SignalClient) Send(message string) error {
	payload := map[string]any{
		"message":    message,
		"number":     c.recipient,
		"recipients": []string{c.recipient},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal signal message: %w", err)
	}

	resp, err := c.httpClient.Post(c.apiURL+"/v1/send", "application/json", bytes.NewReader(body))
	if err != nil {
		slog.Error("signal send failed", "error", err)
		return fmt.Errorf("signal send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("signal send returned %d", resp.StatusCode)
	}

	return nil
}
