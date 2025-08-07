package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
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

	// Campos para el manejo de lotes
	batchInterval time.Duration
	batchSize     int
	mu            sync.Mutex // Para asegurar concurrencia segura en el acceso a los buffers

	streams map[string]*stream // Buffer para los logs antes de ser enviados

	stop chan struct{} // Canal para detener el worker
}

func NewLokiClient(endpoint string, timeout, batchInterval time.Duration, batchSize int) *LokiClient {
	c := &LokiClient{
		endpoint: endpoint,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		batchInterval: batchInterval,
		batchSize:     batchSize,
		streams:       make(map[string]*stream),
		stop:          make(chan struct{}),
	}

	// Iniciar el worker en segundo plano
	go c.startWorker()

	return c
}

// startWorker gestiona el envío periódico de los logs acumulados.
func (c *LokiClient) startWorker() {
	ticker := time.NewTicker(c.batchInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.Flush()
		case <-c.stop:
			// Al detenerse, se envía el último lote de logs
			c.Flush()
			return
		}
	}
}

// PushLog recibe una línea de log y las etiquetas, y la añade al buffer.
func (c *LokiClient) Send(job, streamName, line string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := fmt.Sprintf("%s-%s", job, streamName)
	if _, ok := c.streams[key]; !ok {
		c.streams[key] = &stream{
			Stream: map[string]string{
				"job":    job,
				"stream": streamName,
			},
			Values: make([][]string, 0, c.batchSize),
		}
	}

	ts := strconv.FormatInt(time.Now().UnixNano(), 10)
	c.streams[key].Values = append(c.streams[key].Values, []string{ts, line})

	// Si se alcanza el tamaño de lote, se envía
	if len(c.streams[key].Values) >= c.batchSize {
		c.Flush()
	}
}

// Flush envía los logs acumulados a Loki.
func (c *LokiClient) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.streams) == 0 {
		return
	}

	var streamsToSend []stream
	for _, s := range c.streams {
		streamsToSend = append(streamsToSend, *s)
	}

	reqBody := pushRequest{Streams: streamsToSend}
	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Printf("Error marshaling push request: %v\n", err)
		return
	}

	req, err := http.NewRequest("POST", c.endpoint, bytes.NewBuffer(jsonBytes))
	if err != nil {
		fmt.Printf("Error building HTTP request: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		fmt.Printf("Error sending to Loki: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		fmt.Printf("Unexpected status code from Loki: %d\n", resp.StatusCode)
	}

	// Reiniciar el buffer después de un envío exitoso o fallido
	c.streams = make(map[string]*stream)
}

// Close detiene el worker de fondo y envía cualquier log pendiente.
func (c *LokiClient) Close() {
	close(c.stop)
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
