package catalyst

import (
	"context"
	"errors"
	"net/http"
	"os/exec"
	"time"
)

// DiscoveryClient reuses the collector's public-IP dialer and redirect denial
// for structured public APIs. It does not read environment proxy credentials.
func DiscoveryClient() *http.Client {
	return &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			DialContext: publicDial, TLSHandshakeTimeout: 10 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second, MaxResponseHeaderBytes: 32 << 10,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("discovery redirects are denied")
		},
	}
}

// CollectDiscoveryWebsite exposes the existing bounded Spider/robots pipeline
// without coupling new-asset discovery to the old USDT symbol policy.
func CollectDiscoveryWebsite(ctx context.Context, archive Archive, binary string, source Source) (Evidence, error) {
	if !identifier.MatchString(source.ID) || len(source.Symbols) != 0 {
		return Evidence{}, errors.New("discovery source needs an ID and no symbol binding")
	}
	path, err := exec.LookPath(binary)
	if err != nil {
		return Evidence{}, errors.New("pinned Spider binary is required; no HTTP fallback")
	}
	c := &Collector{Archive: archive, client: DiscoveryClient(), spiderBinary: path}
	e, _, err := c.fetch(ctx, source)
	return e, err
}
