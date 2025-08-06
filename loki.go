package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Estructuras para el payload de push
type pushRequest struct {
	Streams []stream `json:"streams"`
}

type stream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

type LokiClient struct {
	endpoint   string
	httpClient *http.Client
}

func NewLokiClient(endpoint string, timeout time.Duration) *LokiClient {
	return &LokiClient{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// Send envía una sola línea de log a Loki, usando labels job y stream.
func (c *LokiClient) Send(job, streamName, line string) error {
	// Timestamp en nanosegundos
	ts := strconv.FormatInt(time.Now().UnixNano(), 10)

	reqBody := pushRequest{
		Streams: []stream{
			{
				Stream: map[string]string{
					"job":    job,
					"stream": streamName,
				},
				Values: [][]string{
					{ts, line},
				},
			},
		},
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal push request: %w", err)
	}

	req, err := http.NewRequest("POST", c.endpoint, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return fmt.Errorf("build HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send to Loki: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}
