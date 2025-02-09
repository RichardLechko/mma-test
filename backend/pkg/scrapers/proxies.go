package scrapers

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// ProxyManager handles proxy rotation and health checking
type ProxyManager struct {
	proxies       []string
	currentIndex  int
	healthyCount  int
	mu            sync.RWMutex
	lastRotation  time.Time
	rotationDelay time.Duration
	client        *http.Client
}

// ProxyStatus represents the health status of a proxy
type ProxyStatus struct {
	URL       string
	Healthy   bool
	LastCheck time.Time
	Latency   time.Duration
}

// NewProxyManager creates a new proxy manager
func NewProxyManager(proxies []string, rotationDelay time.Duration) *ProxyManager {
	if len(proxies) == 0 {
		return nil
	}

	return &ProxyManager{
		proxies:       proxies,
		currentIndex:  0,
		healthyCount:  len(proxies),
		rotationDelay: rotationDelay,
		lastRotation:  time.Now(),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetProxy returns the next healthy proxy in the rotation
func (pm *ProxyManager) GetProxy() (string, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if len(pm.proxies) == 0 {
		return "", fmt.Errorf("no proxies available")
	}

	if pm.healthyCount == 0 {
		return "", fmt.Errorf("no healthy proxies available")
	}

	// Check if we need to rotate based on time
	if time.Since(pm.lastRotation) >= pm.rotationDelay {
		pm.rotate()
	}

	return pm.proxies[pm.currentIndex], nil
}

// rotate moves to the next proxy in the list
func (pm *ProxyManager) rotate() {
	pm.currentIndex = (pm.currentIndex + 1) % len(pm.proxies)
	pm.lastRotation = time.Now()
}

// AddProxy adds a new proxy to the pool
func (pm *ProxyManager) AddProxy(proxy string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Validate proxy URL
	_, err := url.Parse(proxy)
	if err != nil {
		return fmt.Errorf("invalid proxy URL: %w", err)
	}

	pm.proxies = append(pm.proxies, proxy)
	pm.healthyCount++
	return nil
}

// RemoveProxy removes a proxy from the pool
func (pm *ProxyManager) RemoveProxy(proxy string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for i, p := range pm.proxies {
		if p == proxy {
			pm.proxies = append(pm.proxies[:i], pm.proxies[i+1:]...)
			pm.healthyCount--
			if pm.currentIndex >= i && pm.currentIndex > 0 {
				pm.currentIndex--
			}
			break
		}
	}
}

// CheckProxyHealth tests if a proxy is working
func (pm *ProxyManager) CheckProxyHealth(proxyURL string) *ProxyStatus {
	start := time.Now()
	status := &ProxyStatus{
		URL:       proxyURL,
		LastCheck: start,
	}

	proxyURLParsed, err := url.Parse(proxyURL)
	if err != nil {
		status.Healthy = false
		return status
	}

	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURLParsed),
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		},
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get("https://www.ufc.com")
	if err != nil {
		status.Healthy = false
		return status
	}
	defer resp.Body.Close()

	status.Healthy = resp.StatusCode == http.StatusOK
	status.Latency = time.Since(start)
	return status
}

// HealthCheckAll checks the health of all proxies
func (pm *ProxyManager) HealthCheckAll(ctx context.Context) []ProxyStatus {
	pm.mu.Lock()
	proxies := make([]string, len(pm.proxies))
	copy(proxies, pm.proxies)
	pm.mu.Unlock()

	var statuses []ProxyStatus
	for _, proxy := range proxies {
		select {
		case <-ctx.Done():
			return statuses
		default:
			status := pm.CheckProxyHealth(proxy)
			statuses = append(statuses, *status)
			
			if !status.Healthy {
				pm.RemoveProxy(proxy)
			}
		}
	}

	return statuses
}

// GetRandomProxy returns a random healthy proxy
func (pm *ProxyManager) GetRandomProxy() (string, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if len(pm.proxies) == 0 {
		return "", fmt.Errorf("no proxies available")
	}

	return pm.proxies[rand.Intn(len(pm.proxies))], nil
}

// GetProxyCount returns the total number of proxies
func (pm *ProxyManager) GetProxyCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return len(pm.proxies)
}

// GetHealthyCount returns the number of healthy proxies
func (pm *ProxyManager) GetHealthyCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.healthyCount
}