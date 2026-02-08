# CLAUDE.md

Webhook listener that spins up ephemeral DigitalOcean droplets as GitHub Actions self-hosted runners.

## Stack
- Go 1.24

## Build & Test
```bash
go build ./...
go test ./...
```

## Notes
- Listens for `workflow_job: queued` webhooks
- Creates s-4vcpu-8gb droplets with cloud-init
- Runners use `--ephemeral` flag and self-destruct after one job
- Watchdog cleanup runs every 15 minutes
- Config in `/etc/github-runners/env`
