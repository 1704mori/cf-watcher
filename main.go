package main

import (
	"fmt"
	"log"
	"os"

	"github.com/1704mori/cf-watcher/cloudflare"
	"github.com/1704mori/cf-watcher/docker"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
)

func main() {
	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Error creating Docker client: %v", err)
	}

	err = docker.MonitorEvents(cli, handleEvent)
	if err != nil {
		log.Fatalf("Error monitoring Docker events: %v", err)
	}
}

func handleEvent(cli *client.Client, event events.Message) {
	if event.Type == "container" && event.Action == "start" {
		// Handle start event logic, including interaction with Cloudflare
		containerDetails, err := docker.ParseContainerDetails(
			cli,
			event.ID,
		)
		if err != nil {
			fmt.Println(
				"[DEBUG]: Could not parse container details:",
				err,
			)
		}
		fmt.Println("[DEBUG]: Container details:", containerDetails)
		configEndpoint := fmt.Sprintf(
			"https://api.cloudflare.com/client/v4/accounts/%s/cfd_tunnel/%s/configurations",
			os.Getenv("CF_ACCOUNT_ID"),
			os.Getenv("CF_TUNNEL_ID"),
		)
		if containerDetails != nil &&
			containerDetails.Labels.Enabled {
			// check if tunnel is already configured for that container
			tunnelConfig, err := cloudflare.FetchTunnelConfig(configEndpoint)
			if err != nil {
				fmt.Println(
					"[DEBUG]: Could not fetch tunnel settings:",
					err,
				)
				// todo: fatal error
			}
			fmt.Println(
				"[DEBUG]: Tunnel settings:",
				tunnelConfig.Result,
			)

			var serviceURL string
			if containerDetails.Labels.Rules.Subdomain != "" &&
				containerDetails.Labels.Rules.Domain != "" {
				serviceURL = fmt.Sprintf(
					"%s.%s",
					containerDetails.Labels.Rules.Subdomain,
					containerDetails.Labels.Rules.Domain,
				)
				if containerDetails.Labels.Rules.Path != nil {
					serviceURL = fmt.Sprintf(
						"%s%s",
						serviceURL,
						*containerDetails.Labels.Rules.Path,
					)
				}
			} else if containerDetails.Labels.Rules.Host != "" && containerDetails.Labels.Rules.Port != "" {
				serviceURL = fmt.Sprintf("http://%s:%s", containerDetails.Labels.Rules.Host, containerDetails.Labels.Rules.Port)
			}

			// Check if the service URL exists in any ingress rule
			foundIngress := false
			for _, ingress := range tunnelConfig.Result.Config.Ingress {
				if ingress.Service == serviceURL {
					fmt.Printf(
						"[DEBUG]: Found existing tunnel configuration for service: %s\n",
						serviceURL,
					)
					foundIngress = true
				}
			}
			if !foundIngress {
				fmt.Printf(
					"[DEBUG]: No existing tunnel configuration found for service: %s\n",
					serviceURL,
				)
				err = cloudflare.CreateRoute(
					containerDetails.Labels.Rules.Subdomain,
					containerDetails.Labels.Rules.Domain,
					containerDetails.Labels.Rules.Host,
					containerDetails.Labels.Rules.Port,
					containerDetails.Labels.Rules.Path,
					containerDetails.Labels.Rules.EnableSocks5,
					configEndpoint,
				)
				fmt.Printf(
					"[DEBUG]: Could not create tunnel public route: %v\n",
					err,
				)
			}
		}

	}
}
