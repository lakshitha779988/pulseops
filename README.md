# PulseOps — Cloud-Native Reliability Lab

A portfolio project demonstrating the delivery and monitoring of a Go service. It includes deliberately simulated failures/latency, Docker Compose, Prometheus alert rules, Grafana metrics dashboards, Loki logs via Grafana Alloy, GitHub Actions CI + container publishing, and a 2-replica local Kubernetes deployment.

**This is a local learning/demo environment, NOT a production stack.** Prometheus alerts are visible in its UI but there is no Alertmanager/notification destination; Loki is ephemeral; the Kubernetes deployment is separate from the Docker Compose observability stack.

## Architecture

```text
Client --HTTP--> Go /work, /healthz, /readyz, /metrics
                 |                          |
                 | /metrics                 | JSON logs in shared Docker volume
                 v                          v
             Prometheus                 Grafana Alloy ----> Loki
                 |                                          |
                 +---------------> Grafana <----------------+

GitHub push -> GitHub Actions (Go test / vet -> Docker build -> GHCR publish)
Docker image -> kind (Kubernetes) 2 replicas + Service + probes
```

## Requirements

- Windows 10/11 with PowerShell, Docker Desktop running **Linux containers** and Docker Compose V2. Git is required for GitHub steps.
- For optional Kubernetes demo: `kubectl` and `kind` installed. Docker Desktop Kubernetes or Minikube can also work, but examples below use kind.
- Docker needs sufficient resources for 5 services (PulseOps, Prometheus, Loki, Alloy, Grafana). You do **not** need local Go installed for Docker Compose.

## 1. Start everything

Open PowerShell in this repository's folder:

```powershell
docker compose up -d --build
docker compose ps
```

First image pulls may take time. The services may need a short startup interval; inspect logs if a container exits:

```powershell
docker compose logs --tail=100 pulseops prometheus loki alloy grafana
```

| Interface | URL | Notes |
| --- | --- | --- |
| Go API | http://localhost:8080 | API overview |
| Prometheus metrics | http://localhost:8080/metrics | Go metrics endpoint |
| Prometheus | http://localhost:9090/targets | `pulseops` should be UP |
| Prometheus alerts | http://localhost:9090/alerts | Rule state |
| Grafana | http://localhost:3000 | username `admin`, password `pulseops-local` (local only) |
| Loki | http://localhost:3100/ready | Log server health |
| Alloy | http://localhost:12345 | Collector status |

Grafana dashboard: **Dashboards → PulseOps → PulseOps | Reliability Lab**. The Prometheus and Loki datasources and dashboard are provisioned automatically. For logs, **Explore → Loki →** `{job="pulseops"}`.

You can change Grafana's local password via `GRAFANA_ADMIN_PASSWORD` in a `.env` file **before the first launch**. Never publish `.env`.

## 2. Run smoke tests

```powershell
./scripts/smoke.ps1
```

Or send requests by hand:

```powershell
curl.exe 'http://localhost:8080/healthz'
curl.exe 'http://localhost:8080/work?delay_ms=100'
curl.exe 'http://localhost:8080/work?delay_ms=650&fail=true'
curl.exe 'http://localhost:8080/metrics'
```

`/work` supports `delay_ms=0..2500` and `fail=true`; the latter intentionally returns HTTP 500. Don't expose these deliberately unsafe simulation options publicly.

## 3. Trigger observable problems

```powershell
./scripts/load.ps1 -Count 25 -DelayMs 600
```

What to expect:

1. The **error rate** and **p95 latency** increase in Grafana. P95 is histogram-estimated, not exact.
2. Prometheus `PulseOpsHighErrorCount` should move into **Firing** shortly after its 5-second evaluation cycle once sufficient errors are recorded. `PulseOpsSlowRequests` requires its threshold for 15s.
3. Loki ingests `simulated_failure` JSON log lines from the shared log volume via Alloy (allow a few seconds).
4. No email/Slack notifications are configured; the alerts are visible in Prometheus only.

If logs don't appear, check `docker compose logs alloy loki` and confirm `docker compose exec pulseops ls -l /logs` contains `app.jsonl`.

## 4. Inspect Go code / tests

- `cmd/pulseops/main.go` — server routes, latency/failure simulation, JSON logs, health probes.
- `cmd/pulseops/metrics.go` — fixed-label Prometheus counter/histogram with no third-party Go dependencies.
- `cmd/pulseops/main_test.go` — health, error path, and metric tests.

On a machine with Go 1.23+:

```powershell
go test ./...
go vet ./...
```

### Why metrics are implemented without a client dependency

This small demo intentionally exports valid Prometheus text metrics directly using Go's standard library so Docker and CI builds have zero Go package downloads. In a real production service, prefer the maintained Prometheus `client_golang` library (default runtime/process metrics, well-tested collectors, metric registry, etc.).

## 5. Demonstrate Kubernetes with kind (optional)

The `k8s/` manifests deploy **only the application**, not the full monitoring stack. The Docker Compose Prometheus does not automatically scrape kind pods; app metrics can be inspected via port-forwarding. Full Kubernetes monitoring would require service discovery or a monitoring Helm stack (future extension).

```powershell
kind create cluster --name pulseops
# If you used docker compose up --build, the pulseops:local image is already built.
# Otherwise: docker build -t pulseops:local .
kind load docker-image pulseops:local --name pulseops
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
kubectl -n pulseops rollout status deployment/pulseops
kubectl -n pulseops get pods
kubectl -n pulseops get service
kubectl -n pulseops port-forward service/pulseops 8081:8080
```

In a **second terminal**:

```powershell
curl.exe 'http://localhost:8081/work?delay_ms=150'
curl.exe 'http://localhost:8081/metrics'
```

The deployment includes 2 replicas, resource requests/limits, non-root user, read-only root filesystem, health probes, and a ClusterIP Service.

To clean up kind:

```powershell
kind delete cluster --name pulseops
```

## 6. CI/CD to GitHub Container Registry

After creating a **public** GitHub repository named `pulseops` (lowercase), run:

```powershell
git init
git add .
git commit -m 'feat: build PulseOps reliability lab'
git branch -M main
git remote add origin https://github.com/lakshitha779988/pulseops.git
git push -u origin main
```

`.github/workflows/ci.yaml` runs `go test`, `go vet`, Docker build on PRs; on pushes to `main` it also publishes `ghcr.io/lakshitha779988/pulseops:latest` using GitHub's automatic `GITHUB_TOKEN` permission. There is **no automatic cloud deployment**; container publishing and Kubernetes deployment are separate. If GHCR package is private by default, change its visibility in the package settings before presenting it publicly.

## 7. Cleanup

```powershell
docker compose down
# Deletes app log data stored in the named volume:
docker compose down -v
```

## Suggested portfolio improvements

- Demo screenshot showing Grafana error/latency charts and Loki failure logs.
- Short recording: successful requests → scripted failures → Prometheus firing alert → logs → Kubernetes rollout.
- Add GitHub issue + PR showing a change passing tests/CI.
- Bonus: Kubernetes-native metrics/log discovery and Alertmanager email/Slack routing, with credentials stored in secrets.

## Constraints

- Local demo only; no authorization, request rate limit, secrets management, or exposure hardening.
- Prometheus monitors Docker Compose PulseOps instance, **not** the separate kind deployment.
- Source logs are only forwarded from Compose instance's shared named volume; kind pods use stdout only.
- Prometheus rules display alert states but do not send notifications without Alertmanager.
