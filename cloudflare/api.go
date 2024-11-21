package cloudflare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type DNSRecordPayload struct {
	Content string `json:"content"`
	Name    string `json:"name"`
	Proxied bool   `json:"proxied"`
	Type    string `json:"type"`
}

type TunnelConfigResponse struct {
	Success  bool         `json:"success"`
	Errors   []string     `json:"errors"`
	Messages []string     `json:"messages"`
	Result   TunnelResult `json:"result"`
}

type TunnelResult struct {
	TunnelID  string       `json:"tunnel_id"`
	Version   int          `json:"version"`
	Config    TunnelConfig `json:"config"`
	Source    string       `json:"source"`
	CreatedAt string       `json:"created_at"`
}

type TunnelConfig struct {
	Ingress     []IngressRule `json:"ingress"`
	WarpRouting struct {
		Enabled bool `json:"enabled"`
	} `json:"warp-routing"`
}

type IngressRule struct {
	Service       string                 `json:"service"`
	Hostname      string                 `json:"hostname,omitempty"`
	OriginRequest map[string]interface{} `json:"originRequest,omitempty"`
}

// CreateDNSRecord creates a DNS record in Cloudflare.
func CreateDNSRecord(subdomain, domain string) error {
	zoneID := os.Getenv("CF_ZONE_ID")
	apiURL := fmt.Sprintf(
		"https://api.cloudflare.com/client/v4/zones/%s/dns_records",
		zoneID,
	)

	content := fmt.Sprintf("%s.cfargotunnel.com", os.Getenv("CF_TUNNEL_ID"))

	payload := DNSRecordPayload{
		Content: content,
		Name:    fmt.Sprintf("%s.%s", subdomain, domain),
		Proxied: true,
		Type:    "CNAME",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal DNS record payload: %w", err)
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Key", os.Getenv("CF_AUTH_KEY"))
	req.Header.Set("X-Auth-Email", os.Getenv("CF_AUTH_EMAIL"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(
			"unexpected response code: %d, body: %s",
			resp.StatusCode,
			string(body),
		)
	}

	return nil
}

// FetchTunnelConfig fetches the current tunnel configuration.
func FetchTunnelConfig(endpoint string) (*TunnelConfigResponse, error) {
	client := &http.Client{}

	// Create a new HTTP request
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add required headers (using environment variables for secrets)
	req.Header.Set("X-Auth-Key", os.Getenv("CF_AUTH_KEY"))
	req.Header.Set("X-Auth-Email", os.Getenv("CF_AUTH_EMAIL"))

	// Perform the HTTP request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	// Check if the response status is OK
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf(
			"unexpected response code: %d, body: %s",
			resp.StatusCode,
			string(body),
		)
	}

	// Parse the response body
	var tunnelConfig TunnelConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&tunnelConfig); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &tunnelConfig, nil
}

// CreateRoute updates the tunnel configuration with a new route
func CreateRoute(
	subdomain, domain, host, port string,
	path *string, // Optional path parameter
	socks5 *bool,
	endpoint string,
) error {
	tunnelConfig, err := FetchTunnelConfig(endpoint)
	if err != nil {
		return fmt.Errorf("failed to fetch tunnel config: %w", err)
	}

	// Check if the configuration already exists
	hostname := fmt.Sprintf("%s.%s", subdomain, domain)

	// Find and store the catch-all rule (last rule without hostname)
	var catchAllRule IngressRule
	validIngress := make([]IngressRule, 0)

	for _, ingress := range tunnelConfig.Result.Config.Ingress {
		if ingress.Hostname == "" {
			catchAllRule = ingress
		} else {
			if ingress.Hostname == hostname {
				fmt.Println("Route already exists")
				return nil
			}
			validIngress = append(validIngress, ingress)
		}
	}

	// If no catch-all rule exists, create a default one
	if catchAllRule.Service == "" {
		catchAllRule = IngressRule{
			Service: "http_status:404",
		}
	}

	// Construct the service URL, including the optional path if provided
	serviceURL := fmt.Sprintf("http://%s:%s", host, port)
	if path != nil && *path != "" {
		serviceURL = fmt.Sprintf(
			"%s/%s",
			serviceURL,
			strings.TrimPrefix(*path, "/"),
		)
	}

	// Create new ingress rule
	originRequest := make(map[string]interface{})
	if socks5 != nil {
		originRequest["proxyType"] = "socks"
	}

	newIngress := IngressRule{
		Service:       serviceURL,
		Hostname:      hostname,
		OriginRequest: originRequest,
	}

	// Create the config payload with the new rule and catch-all rule at the end
	payload := struct {
		Config TunnelConfig `json:"config"`
	}{
		Config: TunnelConfig{
			Ingress:     append(append(validIngress, newIngress), catchAllRule),
			WarpRouting: tunnelConfig.Result.Config.WarpRouting,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal route payload: %w", err)
	}

	req, err := http.NewRequest("PUT", endpoint, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Key", os.Getenv("CF_AUTH_KEY"))
	req.Header.Set("X-Auth-Email", os.Getenv("CF_AUTH_EMAIL"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf(
			"unexpected response code: %d, body: %s",
			resp.StatusCode,
			string(body),
		)
	}

	fmt.Println("Route created successfully")
	return nil
}
