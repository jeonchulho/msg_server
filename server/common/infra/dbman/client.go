package dbman

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const BasePath = "/api/internal/v1/db"

const (
	defaultHTTPTimeout      = 5 * time.Second
	defaultFailThreshold    = 3
	defaultEndpointCooldown = 10 * time.Second
)

type Client struct {
	endpoints []string
	http      *http.Client
	next      uint32

	failThreshold    int
	endpointCooldown time.Duration

	mu         sync.Mutex
	failureCnt map[string]int
	cooldownTo map[string]time.Time
}

func NewClient(endpoint string) *Client {
	return NewClientWithEndpoints(endpoint)
}

func NewClientWithEndpoints(endpoints ...string) *Client {
	normalized := normalizeEndpoints(endpoints)
	timeout := durationFromEnvMillis("DBMAN_HTTP_TIMEOUT_MS", defaultHTTPTimeout)
	failThreshold := intFromEnv("DBMAN_FAIL_THRESHOLD", defaultFailThreshold)
	endpointCooldown := durationFromEnvMillis("DBMAN_COOLDOWN_MS", defaultEndpointCooldown)
	return &Client{
		endpoints:        normalized,
		http:             &http.Client{Timeout: timeout},
		failThreshold:    failThreshold,
		endpointCooldown: endpointCooldown,
		failureCnt:       make(map[string]int, len(normalized)),
		cooldownTo:       make(map[string]time.Time, len(normalized)),
	}
}

func (c *Client) Post(ctx context.Context, path string, payload any, out any) error {
	if len(c.endpoints) == 0 {
		return fmt.Errorf("dbman endpoint is not configured")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	normalizedPath := path
	if !strings.HasPrefix(normalizedPath, "/") {
		normalizedPath = "/" + normalizedPath
	}

	start := int(atomic.AddUint32(&c.next, 1)-1) % len(c.endpoints)
	var lastErr error
	for offset := 0; offset < len(c.endpoints); offset++ {
		endpoint := c.endpoints[(start+offset)%len(c.endpoints)]
		if c.isCoolingDown(endpoint, time.Now()) {
			continue
		}
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+normalizedPath, bytes.NewBuffer(body))
		if reqErr != nil {
			lastErr = reqErr
			c.onFailure(endpoint, time.Now())
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, doErr := c.http.Do(req)
		if doErr != nil {
			lastErr = fmt.Errorf("dbman request failed endpoint=%s: %w", endpoint, doErr)
			c.onFailure(endpoint, time.Now())
			continue
		}

		if resp.StatusCode >= 500 {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("dbman status %d endpoint=%s", resp.StatusCode, endpoint)
			c.onFailure(endpoint, time.Now())
			continue
		}
		if resp.StatusCode >= 300 {
			_ = resp.Body.Close()
			return fmt.Errorf("dbman status %d endpoint=%s", resp.StatusCode, endpoint)
		}

		decodeErr := json.NewDecoder(resp.Body).Decode(out)
		_ = resp.Body.Close()
		if decodeErr != nil {
			c.onFailure(endpoint, time.Now())
			return decodeErr
		}
		c.onSuccess(endpoint)
		return nil
	}

	if lastErr == nil {
		return fmt.Errorf("dbman request failed")
	}
	return lastErr
}

func normalizeEndpoints(endpoints []string) []string {
	result := make([]string, 0, len(endpoints))
	seen := map[string]struct{}{}
	for _, endpoint := range endpoints {
		normalized := strings.TrimRight(strings.TrimSpace(endpoint), "/")
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func (c *Client) isCoolingDown(endpoint string, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	until, ok := c.cooldownTo[endpoint]
	if !ok {
		return false
	}
	if now.After(until) {
		delete(c.cooldownTo, endpoint)
		return false
	}
	return true
}

func (c *Client) onFailure(endpoint string, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := c.failureCnt[endpoint] + 1
	c.failureCnt[endpoint] = count
	if count >= c.failThreshold {
		c.cooldownTo[endpoint] = now.Add(c.endpointCooldown)
		c.failureCnt[endpoint] = 0
	}
}

func (c *Client) onSuccess(endpoint string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failureCnt[endpoint] = 0
	delete(c.cooldownTo, endpoint)
}

func intFromEnv(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func durationFromEnvMillis(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return time.Duration(n) * time.Millisecond
}
