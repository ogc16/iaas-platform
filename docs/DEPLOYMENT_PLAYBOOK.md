# Deploying IaaS Platform: A Production Playbook

*Learn how to go from `docker compose up` to production-grade deployments on Railway, Kubernetes, and beyond.*

> **Follow-up to:** [Building a Multi-Tenant IaaS Platform in Go](https://dev.to/ogc16/learn-how-to-build-a-cloud-control-panel-with-authentication-billing-and-resource-management--530e)

---

## Overview

Last week's post showed you the architecture of a production-ready IaaS platform. Today, we're going practical: **how to actually run it in production**.

IaaS Platform ships with multiple deployment options because different teams have different constraints:

- **Local dev** — `docker compose` (5 seconds to running)
- **Single host** — systemd + PostgreSQL (small team, Hetzner/Linode)
- **Containers** — Docker image + Railway/Render (zero-ops, scales easily)
- **Kubernetes** — StatefulSet + Secrets (enterprise teams)
- **Infrastructure-as-Code** — Terraform (repeatable, auditable)

This post walks through each, with real configs you can copy-paste.

---

## Part 1: Local Development (5 Minutes)

Start here. No thinking required.

### The Fastest Possible Start

```bash
git clone https://github.com/ogc16/iaas-platform.git
cd iaas-platform
docker compose up -d
go run ./cmd/server
```

**What just happened:**
- PostgreSQL 16 started in a Docker container (listening on `localhost:5432`)
- Server migrations ran automatically
- Control plane listening on `http://localhost:8080`
- Dashboard loaded at `/`
- API ready at `/api/v1`

### What's Inside `docker-compose.yml`

```yaml
services:
  postgres:
    image: postgres:16-alpine
    ports:
      - "5432:5432"
    environment:
      POSTGRES_USER: iaas
      POSTGRES_PASSWORD: iaas
      POSTGRES_DB: iaas
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U iaas -d iaas"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  pgdata:
```

**Key takeaways:**
- Alpine Linux keeps the image tiny (~150 MB)
- Health checks ensure readiness probes pass
- Volume persists data across container restarts
- Default credentials are fine for local dev (but NOT production)

---

## Part 2: Single-Host Deployment (1 Hour Setup)

You're ready to show this to early users. Use a cheap VPS: **$5/month on Linode or Hetzner**.

### Step 1: Provision the Host

```bash
# SSH into a fresh Ubuntu 22.04 server
ssh root@your-server-ip

# Update everything
apt-get update && apt-get upgrade -y

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sh get-docker.sh

# Install PostgreSQL (system package, NOT container)
apt-get install -y postgresql postgresql-contrib

# Create DB user for the app
sudo -u postgres psql <<EOF
CREATE USER iaas WITH PASSWORD 'YOUR_SECURE_PASSWORD_HERE';
CREATE DATABASE iaas OWNER iaas;
GRANT ALL PRIVILEGES ON DATABASE iaas TO iaas;
EOF

# Verify connection
psql -h localhost -U iaas -d iaas -c "\dt"
```

### Step 2: Deploy the Binary

```bash
# Download v0.1.0 release binary
wget https://github.com/ogc16/iaas-platform/releases/download/v0.1.0/iaas-platform-server-linux-amd64
chmod +x iaas-platform-server-linux-amd64

# Create a user to run the service
useradd -r -s /bin/false iaas-platform

# Move binary to a standard location
mkdir -p /opt/iaas-platform/bin
mv iaas-platform-server-linux-amd64 /opt/iaas-platform/bin/server
chown -R iaas-platform:iaas-platform /opt/iaas-platform
```

### Step 3: Create a systemd Service

Save this to `/etc/systemd/system/iaas-platform.service`:

```ini
[Unit]
Description=IaaS Platform Control Plane
After=network-online.target postgresql.service
Wants=network-online.target

[Service]
Type=simple
User=iaas-platform
Group=iaas-platform

# Load environment variables from a file
EnvironmentFile=/etc/iaas-platform/env

ExecStart=/opt/iaas-platform/bin/server

# Auto-restart if the process crashes
Restart=on-failure
RestartSec=5

# Limits
LimitNOFILE=65536
TimeoutStopSec=30

[Install]
WantedBy=multi-user.target
```

### Step 4: Configure Environment Variables

Create `/etc/iaas-platform/env`:

```bash
# Production hardening
ENV=production
PORT=8080

# Database (use the user/password from Step 1)
DATABASE_URL=postgres://iaas:YOUR_SECURE_PASSWORD_HERE@localhost:5432/iaas?sslmode=require

# Generate a strong secret: openssl rand -hex 32
JWT_SECRET=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

# Password hashing
BCRYPT_COST=12

# Email (optional; without SMTP, password-reset links are logged to stdout)
SMTP_HOST=smtp.sendgrid.net
SMTP_PORT=587
SMTP_USERNAME=apikey
SMTP_PASSWORD=SG.xxxxxxxxxxxxx
SMTP_FROM=noreply@yourdomain.com

# Base URL for password-reset links
APP_BASE_URL=https://iaas.yourdomain.com
```

Then restrict read access: `chmod 600 /etc/iaas-platform/env`

### Step 5: Enable and Start

```bash
systemctl daemon-reload
systemctl enable iaas-platform
systemctl start iaas-platform
systemctl status iaas-platform   # Verify it's running
```

### Step 6: Nginx Reverse Proxy

Put this in `/etc/nginx/sites-available/iaas`:

```nginx
upstream iaas_platform {
    server 127.0.0.1:8080;
}

# HTTP → HTTPS redirect
server {
    listen 80;
    server_name iaas.yourdomain.com;
    return 301 https://$server_name$request_uri;
}

# HTTPS server
server {
    listen 443 ssl http2;
    server_name iaas.yourdomain.com;

    # Use Let's Encrypt (or your CA)
    ssl_certificate /etc/letsencrypt/live/iaas.yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/iaas.yourdomain.com/privkey.pem;

    # Security headers (some added by our app, but reinforce here)
    add_header Strict-Transport-Security "max-age=63072000; includeSubDomains; preload" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "DENY" always;
    add_header Referrer-Policy "no-referrer" always;

    # Request body size limit (prevents DoS; issue #13 will add this to the app)
    client_max_body_size 1M;

    # Proxy to our app
    location / {
        proxy_pass http://iaas_platform;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Timeouts for long-polling (health checks, etc.)
        proxy_read_timeout 30s;
        proxy_connect_timeout 10s;
    }
}
```

### Step 7: Get an SSL Certificate

```bash
apt-get install -y certbot python3-certbot-nginx
certbot certonly --nginx -d iaas.yourdomain.com

# Auto-renew
systemctl enable certbot.timer
```

### Step 8: Verify It Works

```bash
# Check the app is running
curl https://iaas.yourdomain.com/healthz
# Should return: OK

# Check readiness (pings the database)
curl https://iaas.yourdomain.com/readyz
# Should return: OK

# Open in browser
open https://iaas.yourdomain.com
```

**Cost:** ~$5/month (VPS) + ~$10/year (domain) + free SSL (Let's Encrypt).

---

## Part 3: Container Deployment (No Server Management)

Use **Railway** or **Render** for zero-ops deployment. We'll show Railway because it's fast.

### Step 1: Containerize the App

The repo includes a multi-stage `Dockerfile`:

```dockerfile
# syntax=docker/dockerfile:1

# --- build stage -------------------------------------------------------------
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server
# The migrate binary is included for staged rollouts: run it before deploying
# the new server binary when you want migrations applied as a separate job.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# --- runtime stage ----------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/server /out/migrate /usr/local/bin/
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/server"]
```

**Why distroless?**
- ✅ Image size: ~30 MB (not 800 MB with Alpine)
- ✅ No shell or package manager = smaller attack surface
- ✅ Runs as non-root by default (`uid 65532`)
- ✅ Security scanning doesn't find phantom vulnerabilities

### Step 2: Pull the Published Image

The `ghcr.io/ogc16/iaas-platform` image is published with every release:

```bash
docker pull ghcr.io/ogc16/iaas-platform:v0.1.0
```

To publish your own fork instead:

```bash
docker build -t ghcr.io/YOUR_GITHUB_USERNAME/iaas-platform:latest .
docker push ghcr.io/YOUR_GITHUB_USERNAME/iaas-platform:latest
```

### Step 3: Deploy to Railway

1. Go to [railway.app](https://railway.app)
2. Click **"New Project"** → **"Deploy from GitHub"**
3. Select your `iaas-platform` fork
4. Railway auto-detects the Dockerfile
5. **Add PostgreSQL 16** plugin
6. Set environment variables:
   ```
   ENV=production
   JWT_SECRET=<generate: openssl rand -hex 32>
   DATABASE_URL=<Railway generates this automatically>
   BCRYPT_COST=12
   ```
7. Deploy button → **Wait 2 minutes**

**That's it.** Railway provides:
- ✅ HTTPS by default (auto-generated domain)
- ✅ PostgreSQL managed for you
- ✅ Auto-scaling
- ✅ $5/month free tier (then $5/gb RAM/month)

**Your app is now live at:** `https://yourdomain-production-xxxx.railway.app`

---

## Part 4: Kubernetes Deployment (Enterprise)

For large teams, Kubernetes gives you **orchestration, auto-scaling, and self-healing**.

### Prerequisites

- A Kubernetes cluster (EKS, GKE, DigitalOcean, or local `minikube`)
- `kubectl` CLI
- `helm` (optional but recommended)

### Step 1: Create Secrets

```bash
# Generate a strong JWT secret
JWT_SECRET=$(openssl rand -hex 32)

# Create a Kubernetes secret
kubectl create secret generic iaas-platform-secrets \
  --from-literal=JWT_SECRET=$JWT_SECRET \
  --from-literal=DATABASE_URL=postgres://iaas:password@postgres-svc:5432/iaas \
  --from-literal=BCRYPT_COST=12 \
  --from-literal=ENV=production
```

### Step 2: Deploy PostgreSQL (if not using managed RDS)

Create `postgres.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: postgres-config
data:
  POSTGRES_USER: iaas
  POSTGRES_DB: iaas

---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-pvc
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres
spec:
  replicas: 1
  selector:
    matchLabels: { app: postgres }
  template:
    metadata:
      labels: { app: postgres }
    spec:
      containers:
      - name: postgres
        image: postgres:16-alpine
        envFrom:
        - configMapRef:
            name: postgres-config
        env:
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: postgres-secrets
              key: password
        ports:
        - containerPort: 5432
        volumeMounts:
        - name: data
          mountPath: /var/lib/postgresql/data
        livenessProbe:
          exec:
            command: ["pg_isready", "-U", "iaas"]
          initialDelaySeconds: 30
          periodSeconds: 10
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: postgres-pvc

---
apiVersion: v1
kind: Service
metadata:
  name: postgres-svc
spec:
  selector: { app: postgres }
  ports:
  - port: 5432
    targetPort: 5432
```

### Step 3: Deploy the App

Create `iaas-platform.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: iaas-platform
  labels: { app: iaas-platform }
spec:
  replicas: 2
  selector:
    matchLabels: { app: iaas-platform }
  template:
    metadata:
      labels: { app: iaas-platform }
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
      containers:
      - name: server
        image: ghcr.io/ogc16/iaas-platform:v0.1.0
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 8080
          name: http
        envFrom:
        - secretRef:
            name: iaas-platform-secrets
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5

---
apiVersion: v1
kind: Service
metadata:
  name: iaas-platform-svc
spec:
  selector: { app: iaas-platform }
  type: LoadBalancer
  ports:
  - port: 80
    targetPort: 8080
    protocol: TCP

---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: iaas-platform-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: iaas-platform
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

### Step 4: Apply and Verify

```bash
# Create namespace
kubectl create namespace iaas

# Apply configs
kubectl apply -f postgres.yaml -n iaas
kubectl apply -f iaas-platform.yaml -n iaas

# Wait for rollout
kubectl rollout status deployment/iaas-platform -n iaas

# Get the LoadBalancer IP
kubectl get svc iaas-platform-svc -n iaas

# Test it
curl http://<LOAD_BALANCER_IP>/healthz
```

**Kubernetes gives you:**
- ✅ Auto-scaling (CPU-based HPA)
- ✅ Self-healing (pod restarts on failure)
- ✅ Rolling updates (zero downtime deploys)
- ✅ Multi-region ready (Federated clusters)

---

## Part 5: Infrastructure-as-Code (Terraform)

For repeatable, auditable infrastructure.

### What's Included

The repo has `examples/terraform/` with configs for **single-host Docker deployment** (great for staging):

```hcl
# examples/terraform/main.tf

terraform {
  required_version = ">= 1.5"
  required_providers {
    docker = {
      source  = "kreuzwerker/docker"
      version = "~> 3.0"
    }
  }
}

provider "docker" {}

# PostgreSQL container
resource "docker_container" "postgres" {
  name  = "iaas-platform-postgres"
  image = docker_image.postgres.image_id
  env = [
    "POSTGRES_USER=iaas",
    "POSTGRES_PASSWORD=${var.database_password}",
    "POSTGRES_DB=iaas",
  ]
  ports {
    internal = 5432
    external = var.database_port
  }
  volumes {
    volume_name    = docker_volume.postgres_data.name
    container_path = "/var/lib/postgresql/data"
  }
}

# App server container
resource "docker_container" "server" {
  name  = "iaas-platform-server"
  image = docker_image.iaas_platform.image_id
  env = [
    "ENV=production",
    "HTTP_ADDR=:${var.server_port}",
    "DATABASE_URL=postgres://iaas:${var.database_password}@postgres:5432/iaas?sslmode=disable",
    "JWT_SECRET=${var.jwt_secret}",
  ]
  ports {
    internal = var.server_port
    external = var.server_port
  }
  depends_on = [docker_container.postgres]
}
```

### Deploy with Terraform

```bash
cd examples/terraform
terraform init
terraform apply -var server_image_tag=v0.1.0
```

**Benefits:**
- ✅ Repeatable (same config → same infrastructure)
- ✅ Auditable (git history of changes)
- ✅ Shareable (colleague runs `terraform apply`, gets identical setup)
- ✅ Destroyable (tear down with `terraform destroy`)

---

## Production Readiness Checklist

Before launching, verify:

### Security
- [ ] `ENV=production` (refuses weak JWT secrets)
- [ ] `JWT_SECRET` is ≥32 bytes random (run: `openssl rand -hex 32`)
- [ ] TLS enabled (HTTPS only, no HTTP)
- [ ] Database password strong
- [ ] SMTP credentials NOT committed to Git (use env or secrets manager)
- [ ] API key hashing enabled (default)

### Operations
- [ ] Health probes working (`/healthz`, `/readyz`)
- [ ] Logs flowing to stdout (JSON structured via slog)
- [ ] Backups scheduled (`pg_dump` daily)
- [ ] Rollback plan documented

### Testing
- [ ] `api-usage.sh` works against production
- [ ] Dashboard loads
- [ ] Can create org, instance, and generate invoice
- [ ] Password reset emails working (if SMTP enabled)

---

## Cost Comparison

| Platform | Cost/Month | Setup Time | Scaling | Best For |
|---|---|---|---|---|
| **Single Host (systemd)** | $5-15 | 1 hour | Manual | Early-stage startups, learning |
| **Railway/Render** | $5-50 | 10 min | Auto | Fast iteration, small teams |
| **Kubernetes (GKE/EKS)** | $100+ | 2 hours | Auto | Enterprise, high traffic |
| **Terraform (staging)** | $10-50 | 30 min | Repeatable | CI/CD integration, testing |

---

## Deployment Workflow (Complete Example)

```bash
# 1. Build and test locally
make test
make race
docker build -t iaas-platform:latest .

# 2. Push to registry
docker tag iaas-platform:latest ghcr.io/ogc16/iaas-platform:v0.1.1
docker push ghcr.io/ogc16/iaas-platform:v0.1.1

# 3. Deploy to staging (Terraform)
cd examples/terraform
terraform apply -var server_image_tag=v0.1.1

# 4. Smoke test
curl http://staging-server/healthz
bash examples/api-usage.sh

# 5. Deploy to production (Railway or K8s)
# Railway: auto-deploys on git push
# K8s: kubectl set image deployment/iaas-platform \
#        server=ghcr.io/ogc16/iaas-platform:v0.1.1

# 6. Verify
curl https://iaas.yourdomain.com/healthz
```

---

## Monitoring in Production

IaaS Platform logs structured JSON (via Go's `slog`). Aggregate with:

- **ELK Stack** — Elasticsearch + Logstash + Kibana
- **Datadog** — SaaS APM (integrates with Kubernetes)
- **Grafana Loki** — Lightweight log aggregation
- **CloudWatch** — If using AWS

Every request includes an `X-Request-ID` for correlation.

**Missing:** Prometheus metrics. That's issue #14; see [ROADMAP.md](https://github.com/ogc16/iaas-platform/blob/master/ROADMAP.md).

---

## Next Steps

1. **Pick your platform** — Start with Railway if unsure
2. **Follow the steps above** — Copy-paste the configs
3. **Test the API** — Run `examples/api-usage.sh`
4. **Explore the dashboard** — Create an org, spin up an instance
5. **Open an issue** — Something missing? File it on GitHub

---

## Troubleshooting

### "healthz returns 503"
Database isn't reachable. Check:
```bash
# Is PostgreSQL running?
docker ps | grep postgres

# Can the app reach it?
curl $DATABASE_URL  # Should fail gracefully, not hang
```

### "502 Bad Gateway"
App crashed. Check logs:
```bash
# If systemd:
journalctl -u iaas-platform -f

# If Docker:
docker logs iaas-platform-server

# If Kubernetes:
kubectl logs -f deployment/iaas-platform -n iaas
```

### "Certificate verification failed"
TLS misconfigured. For development:
```bash
curl --insecure https://localhost:8080/healthz
```
For production, ensure Let's Encrypt cert is valid:
```bash
openssl x509 -in /etc/letsencrypt/live/yourdomain.com/fullchain.pem -text -noout
```

---

## What's Next?

This series has covered:

1. **[Part 1 — Architecture](https://dev.to/ogc16/learn-how-to-build-a-cloud-control-panel-with-authentication-billing-and-resource-management--530e)**: Multi-tenant design, billing engine, async lifecycle
2. **[Part 2 — Deployment](https://dev.to/ogc16/deploying-iaas-platform-a-production-playbook-421f)** (this post): Local, single-host, containerized, K8s
3. **Part 3 (coming soon)** - Scaling beyond v0.1.0: Prometheus metrics, webhook notifications, real compute backends

---

## Join the Community

The IaaS Platform is open source and growing:

- ⭐ [GitHub](https://github.com/ogc16/iaas-platform)
- 💬 [Discussions](https://github.com/ogc16/iaas-platform/discussions)
- 🐛 [Open Issues](https://github.com/ogc16/iaas-platform/issues) (5 good-first-issues for contributors)

**Have deployment questions?** Reply in the comments below or open an issue.

Happy deploying! 🚀
