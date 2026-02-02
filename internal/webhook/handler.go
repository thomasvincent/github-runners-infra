package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/thomasvincent/github-runners-infra/internal/digitalocean"
	gh "github.com/thomasvincent/github-runners-infra/internal/github"
)

// WorkflowJobEvent represents the GitHub workflow_job webhook payload.
type WorkflowJobEvent struct {
	Action      string      `json:"action"`
	WorkflowJob WorkflowJob `json:"workflow_job"`
	Org         *OrgInfo    `json:"organization,omitempty"`
	Repo        RepoInfo    `json:"repository"`
}

type WorkflowJob struct {
	ID     int64    `json:"id"`
	Name   string   `json:"name"`
	Labels []string `json:"labels"`
}

type OrgInfo struct {
	Login string `json:"login"`
}

type RepoInfo struct {
	FullName string `json:"full_name"`
	Name     string `json:"name"`
	Owner    struct {
		Login string `json:"login"`
	} `json:"owner"`
}

// Handler processes incoming GitHub webhooks.
type Handler struct {
	webhookSecret []byte
	githubApp     *gh.App
	doClient      *digitalocean.Client
	doToken       string
	requiredLabel string
	runnerVersion string
}

// Config holds handler configuration.
type Config struct {
	WebhookSecret []byte
	GitHubApp     *gh.App
	DOClient      *digitalocean.Client
	DOToken       string
	RequiredLabel string
	RunnerVersion string
}

// NewHandler creates a new webhook handler.
func NewHandler(cfg Config) *Handler {
	label := cfg.RequiredLabel
	if label == "" {
		label = "self-hosted"
	}
	version := cfg.RunnerVersion
	if version == "" {
		version = "2.321.0"
	}
	return &Handler{
		webhookSecret: cfg.WebhookSecret,
		githubApp:     cfg.GitHubApp,
		doClient:      cfg.DOClient,
		doToken:       cfg.DOToken,
		requiredLabel: label,
		runnerVersion: version,
	}
}

// ServeHTTP handles webhook requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	sig := r.Header.Get("X-Hub-Signature-256")
	if !gh.VerifyWebhookSignature(body, sig, h.webhookSecret) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	if eventType != "workflow_job" {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ignored event type")
		return
	}

	var event WorkflowJobEvent
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if event.Action != "queued" {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ignored action")
		return
	}

	if !h.hasRequiredLabel(event.WorkflowJob.Labels) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "no matching labels")
		return
	}

	go h.provisionRunner(event)

	w.WriteHeader(http.StatusAccepted)
	fmt.Fprint(w, "runner provisioning started")
}

func (h *Handler) hasRequiredLabel(labels []string) bool {
	for _, l := range labels {
		if strings.EqualFold(l, h.requiredLabel) {
			return true
		}
	}
	return false
}

func (h *Handler) provisionRunner(event WorkflowJobEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	owner := event.Repo.Owner.Login
	repo := event.Repo.Name

	runnerToken, err := h.githubApp.GenerateRepoRunnerToken(owner, repo)
	if err != nil {
		log.Printf("ERROR: generate runner token for %s/%s: %v", owner, repo, err)
		return
	}

	runnerName := fmt.Sprintf("eph-%s-%d-%d", repo, event.WorkflowJob.ID, time.Now().Unix())
	if len(runnerName) > 63 {
		runnerName = runnerName[:63]
	}

	labels := strings.Join(event.WorkflowJob.Labels, ",")

	params := digitalocean.RunnerParams{
		RunnerName:    runnerName,
		RunnerToken:   runnerToken,
		RunnerLabels:  labels,
		RunnerOrg:     owner,
		RunnerRepo:    fmt.Sprintf("%s/%s", owner, repo),
		DOToken:       h.doToken,
		RunnerVersion: h.runnerVersion,
	}

	droplet, err := h.doClient.CreateRunner(ctx, params)
	if err != nil {
		log.Printf("ERROR: create runner droplet for job %d: %v", event.WorkflowJob.ID, err)
		return
	}

	log.Printf("Provisioned runner %s (droplet ID: %d) for %s/%s job %d",
		runnerName, droplet.ID, owner, repo, event.WorkflowJob.ID)
}
