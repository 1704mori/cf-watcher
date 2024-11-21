# CloudWatch: Docker Cloudflare Tunnel Automator 🚀

**I will update it according to my needs and improvements that I find necessary for the time being.**

Automatically configure Cloudflare Tunnels for your Docker containers using simple container labels.

## Overview

CloudWatch is a tool that monitors Docker container events and automatically configures Cloudflare Tunnels based on container labels. It enables automatic subdomain creation and routing for your Docker containers through Cloudflare's Zero Trust network.

## Features

- 🔄 Automatic tunnel configuration when containers start
- 🏷️ Simple container labeling system
- 🔒 Cloudflare Zero Trust integration
- 🌐 Automatic subdomain creation
- 🔌 SOCKS5 proxy support
- 🚦 Automatic network detection
- 🛡️ Works with existing Cloudflare Tunnel configurations
- :( Currently only http tunnel type is supported (is this even a feature?)

## Getting Started

### Prerequisites

- Docker
- A Cloudflare account with:
  - An existing Cloudflare Tunnel, created through Docker
  - API access
  - A domain managed by Cloudflare

### Environment Variables

```env
CF_AUTH_KEY=your_cloudflare_api_key
CF_AUTH_EMAIL=your_cloudflare_email
CF_ACCOUNT_ID=your_cloudflare_account_id
CF_TUNNEL_ID=your_cloudflare_tunnel_id
```

### Container Labels

Configure your containers with these labels to enable automatic tunnel routing:

```yaml
labels:
  # Enable CloudWatch for this container
  cf_watch.enabled: "true"
  
  # Optional: Specify the network containing your cloudflared container (if not specified it will try to find cloudflared container's network)
  cf_watch.cf_network: "cloudflare_network"
  
  # Configure routing rules
  cf_watch.rules.subdomain: "myapp"     # Required: Subdomain for the route
  cf_watch.rules.domain: "example.com"   # Required: Domain for the route
  cf_watch.rules.host: "container_name"  # Required: Container hostname/IP
  cf_watch.rules.port: "8080"           # Required: Container port
  cf_watch.rules.path: "/api"           # Optional: Path to route
```

### Docker Compose Example

```yaml
version: '3'
services:
  myapp:
    image: nginx
    labels:
      cf_watch.enabled: "true"
      cf_watch.rules.subdomain: "myapp"
      cf_watch.rules.domain: "example.com"
      cf_watch.rules.host: "myapp"
      cf_watch.rules.port: "80"

  cloudwatch:
    image: cloudwatch
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - CF_AUTH_KEY=your_key
      - CF_AUTH_EMAIL=your_email
      - CF_ACCOUNT_ID=your_account_id
      - CF_TUNNEL_ID=your_tunnel_id
```

## How It Works

1. CloudWatch monitors Docker container events using the Docker API
2. When a container starts, it checks for CloudWatch-specific labels
3. If enabled, it:
   - Validates the container's configuration
   - Checks for existing tunnel routes
   - Creates new DNS records if needed
   - Updates the Cloudflare Tunnel configuration
   - Maintains existing tunnel configurations

## Development

### Building from Source

```bash
git clone https://github.com/1704mori/cf-watcher.git
cd cf-watcher
go build
```

### Running Tests
todo

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Cloudflare API
- Go Docker SDK

## Troubleshooting

### Common Issues

1. **Container not being detected:**
   - Verify the Docker socket is mounted correctly
   - Check if CloudWatch has appropriate permissions

2. **Tunnel configuration fails:**
   - Verify your Cloudflare credentials
   - Ensure the tunnel ID is correct
   - Check the container labels are properly configured

3. **Network detection issues:**
   - Explicitly set the `cf_watch.cf_network` label
   - Ensure the cloudflared container is running

## Security Considerations

- Store Cloudflare credentials securely
- Use restricted API tokens when possible
- Keep the Docker socket secure
- Monitor tunnel access logs

## Need Help?

- Create an issue in the GitHub repository
- Check the Cloudflare documentation
- Review the Docker API documentation
