package docker

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

type Container struct {
	Name      string
	IPAddress string
	Labels    Labels
}

type Labels struct {
	Enabled   bool
	CFNetwork string
	Rules     Rule
}

type Rule struct {
	Subdomain    string
	Domain       string
	EnableSocks5 *bool
	Path         *string
	Type         string
	Host         string
	Port         string
}

// MonitorEvents listens for Docker events.
func MonitorEvents(
	cli *client.Client,
	handler func(cli *client.Client, event events.Message),
) error {
	ctx := context.Background()
	eventOptions := events.ListOptions{}
	eventStream, errs := cli.Events(ctx, eventOptions)

	for {
		select {
		case event := <-eventStream:
			handler(cli, event)
		case err := <-errs:
			return fmt.Errorf("error receiving events: %w", err)
		}
	}
}

// ParseContainerDetails extracts details of a container.
func ParseContainerDetails(
	cli *client.Client,
	containerID string,
) (*Container, error) {
	ctx := context.Background()

	containerJSON, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf(
			"could not inspect container %s: %w",
			containerID,
			err,
		)
	}

	labels, err := parseLabels(cli, &containerJSON)
	if err != nil {
		return nil, fmt.Errorf("error parsing labels: %w", err)
	}

	return &Container{
		Name:      containerJSON.Name,
		IPAddress: containerJSON.NetworkSettings.IPAddress,
		Labels:    labels,
	}, nil
}

// Helper function: parse labels
func parseLabels(
	cli *client.Client,
	containerJSON *types.ContainerJSON,
) (Labels, error) {
	var cfEnabled bool
	var cfNetwork string

	labels := containerJSON.Config.Labels
	networks := containerJSON.NetworkSettings.Networks

	enabled, exists := labels["cf_watcher.enabled"]
	if exists {
		if enabled == "true" {
			cfEnabled = true
		} else if enabled == "false" {
			cfEnabled = false
		}
	}

	if !cfEnabled {
		return Labels{}, fmt.Errorf(
			"cf_watcher is not enabled for container %s",
			containerJSON.Name,
		)
	}

	// Validate cf_watcher.cf_network
	if network, exists := labels["cf_watcher.cf_network"]; exists {
		cfNetwork = network
	} else {
		// Fallback: Find a suitable network
		cfNetwork = findCFNetwork(cli, networks)
		if cfNetwork == "" {
			return Labels{}, fmt.Errorf("cf_watcher.cf_network is required but could not be determined automatically, container: %s", containerJSON.Name)
		}
	}

	// Parse rules
	ruleData := make(map[string]interface{})
	for key, value := range labels {
		if strings.HasPrefix(key, "cf_watcher.rules") {
			parts := strings.Split(key, ".")
			if len(parts) > 2 {
				property := parts[2]
				ruleData[property] = value
			}
		}
	}

	rule := Rule{
		Subdomain: ruleData["subdomain"].(string),
		Domain:    ruleData["domain"].(string),
		Type:      ruleData["type"].(string),
		Host:      ruleData["host"].(string),
		Port:      ruleData["port"].(string),
	}

	if path, exists := ruleData["path"]; exists {
		_path := path.(*string)
		rule.Path = _path
	}

	if socks, exists := ruleData["socks5"]; exists {
		_socks5 := socks.(*bool)
		rule.EnableSocks5 = _socks5
	}

	return Labels{
		Enabled:   cfEnabled,
		CFNetwork: cfNetwork,
		Rules:     rule,
	}, nil
}

// Helper function: find Cloudflare network
func findCFNetwork(
	cli *client.Client,
	networks map[string]*network.EndpointSettings,
) string {
	ctx := context.Background()

	for networkName := range networks {
		// List containers connected to the network
		containers, err := cli.ContainerList(
			ctx,
			container.ListOptions{All: true},
		)
		if err != nil {
			log.Printf(
				"Error listing containers for network %s: %v",
				networkName,
				err,
			)
			continue
		}

		// Look for a "cloudflared" container
		for _, container := range containers {
			if strings.Contains(container.Image, "cloudflared") {
				log.Printf(
					"Found cloudflared container in network %s",
					networkName,
				)
				return networkName
			}
		}
	}

	return ""
}
