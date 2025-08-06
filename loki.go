package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

// WaitReady intenta hacer GET al endpoint /ready hasta maxRetries veces,
// pausando "interval" entre cada intento, y retorna error si no obtiene 200 OK.
func (c *LokiClient) WaitReady(maxRetries int, interval time.Duration) error {
	readyURL := c.endpoint
	if u, err := url.Parse(c.endpoint); err == nil {
		u.Path = "/ready"
		readyURL = u.String()
	}
	client := &http.Client{Timeout: interval}
	for i := 0; i < maxRetries; i++ {
		resp, err := client.Get(readyURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(interval)
	}
	return fmt.Errorf("loki no respondió en %s tras %d intentos", readyURL, maxRetries)
}
