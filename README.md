# GitHub Actions Ephemeral Runners on DigitalOcean

A Go webhook listener that provisions ephemeral DigitalOcean droplets as GitHub Actions self-hosted runners. Runners self-destruct after completing a single job. Zero cost when idle.

## Architecture

```
GitHub webhook (workflow_job: queued)
  → Go webhook listener (on $6/mo DO droplet)
    → Creates s-4vcpu-8gb droplet with cloud-init
      → Installs Docker + runner + Chef deps
      → Registers as ephemeral runner (--ephemeral)
      → Runs one job, auto-unregisters
      → Self-destructs via DO API
```

## Prerequisites

- Go 1.22+
- A GitHub App with `administration:write` permission (for runner registration)
- A DigitalOcean API token

## Setup

### 1. Create a GitHub App

1. Go to your org settings → Developer settings → GitHub Apps → New GitHub App
2. Set webhook URL to `https://your-domain.com/webhook`
3. Set permissions: **Repository administration: Write**
4. Subscribe to events: **Workflow job**
5. Install on your org/repos

### 2. Configure Environment

Create `/etc/github-runners/env`:

```bash
GITHUB_APP_ID=123456
GITHUB_INSTALLATION_ID=789012
GITHUB_APP_PRIVATE_KEY="-----BEGIN RSA PRIVATE KEY-----\n..."
GITHUB_WEBHOOK_SECRET=your-webhook-secret
DIGITALOCEAN_TOKEN=dop_v1_...
DO_REGION=nyc3
DO_SIZE=s-4vcpu-8gb
REQUIRED_LABEL=self-hosted
```

### 3. Build & Deploy

```bash
make build
make deploy  # copies binaries and systemd units to runner-host
```

### 4. Update Workflow Files

For integration test jobs in your cookbooks:

```yaml
jobs:
  integration:
    runs-on: [self-hosted, chef]
```

Keep lint/unit jobs on `ubuntu-latest`.

## Cost

- Webhook listener: ~$6/mo (s-1vcpu-1gb always-on)
- Runners: ~$0.071/hr per droplet, only while running
- ~20 jobs/day × 15 min avg ≈ $10.65/mo
- **Total: ~$17/mo**

## Cleanup

A watchdog runs every 15 minutes and deletes runner droplets older than 60 minutes to catch any orphaned instances.

## Development

```bash
# Run tests
go test ./...

# Run locally (requires env vars)
go run cmd/webhook/main.go
```

## License

MIT
