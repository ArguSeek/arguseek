# ArguSeek Deployment Guide

This guide covers deploying ArguSeek to Google Cloud Run using the automated deployment system.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Configuration](#configuration)
- [Deployment Workflow](#deployment-workflow)
- [Traffic Routing Verification](#traffic-routing-verification)
- [Rollback](#rollback)

## Overview

ArguSeek includes a comprehensive automated deployment system that handles:

- **Docker image building** for Cloud Run (`linux/amd64`)
- **Image pushing** to Google Container Registry (GCR)
- **Cloud Run deployment** with configured resource limits
- **Traffic routing verification** with exponential backoff (handles Cloud Run's 30-60s routing delay)
- **Post-deployment validation** via integrated QA harness
- **Dry-run preview** mode for safety

The deployment system uses environment-specific configuration files and follows a **safety-first workflow**: preview → review → execute.

## Prerequisites

### Required Tools

- **Go 1.23+** (for building the deployment tool)
- **Docker** (for building container images)
- **Google Cloud SDK (gcloud)** - [Install guide](https://cloud.google.com/sdk/docs/install)
- **GitHub CLI (gh)** (optional, for release management)

### Google Cloud Setup

1. **Create a GCP Project**:
   ```bash
   gcloud projects create your-project-id
   gcloud config set project your-project-id
   ```

2. **Enable Required APIs**:
   ```bash
   gcloud services enable run.googleapis.com
   gcloud services enable containerregistry.googleapis.com
   ```

3. **Authenticate Docker with GCR**:
   ```bash
   gcloud auth configure-docker
   ```

4. **Set up Default Application Credentials**:
   ```bash
   gcloud auth application-default login
   ```

### API Keys

You'll need API keys for ArguSeek to function. See the main [README.md](README.md#configuration) for details:

- `GOOGLE_API_KEY` (required) - [Get key](https://developers.google.com/custom-search/v1/introduction)
- `GOOGLE_CSE_ID` (required) - [Create engine](https://programmablesearchengine.google.com/)
- `GEMINI_API_KEY` (optional) - [Get key](https://ai.google.dev/)

## Configuration

### Step 1: Create Environment Configuration Files

ArguSeek uses YAML files for infrastructure configuration and `.env` files for secrets.

**Create `config/dev.yaml`** (copy from example):
```bash
cp config/dev.yaml.example config/dev.yaml
```

Edit `config/dev.yaml`:
```yaml
project_id: "your-gcp-project-id"
region: "us-central1"
service_name: "arguseek-dev"
service_account: ""  # Optional: specify service account email
google_cse_id: "your-custom-search-engine-id"

runtime:
  memory: "512Mi"
  cpu: 1
  timeout: 300
  concurrency: 80
  min_instances: 0  # 0 for cost savings in dev
  max_instances: 3
```

**Create `config/.env.dev`** (copy from example):
```bash
cp config/.env.dev.example config/.env.dev
```

Edit `config/.env.dev`:
```bash
GOOGLE_API_KEY=your-actual-google-api-key
GEMINI_API_KEY=your-actual-gemini-api-key
```

**For production**, repeat the same steps with `prod.yaml` and `.env.prod`:
```bash
cp config/prod.yaml.example config/prod.yaml
cp config/.env.prod.example config/.env.prod
# Edit both files with production values
```

### Step 2: Secure Your Secrets

**Important**: Never commit actual secrets to version control.

The `.gitignore` should already exclude:
```
config/.env.dev
config/.env.prod
config/dev.yaml
config/prod.yaml
```

Only the `.example` files are tracked in git.

### Configuration Parameters Explained

#### YAML Configuration

| Parameter | Description | Example Values |
|-----------|-------------|----------------|
| `project_id` | Your GCP project ID | `"my-project-123"` |
| `region` | Cloud Run region | `"us-central1"`, `"europe-west1"` |
| `service_name` | Cloud Run service name | `"arguseek-dev"`, `"arguseek-prod"` |
| `service_account` | Service account email (optional) | `"sa@project.iam.gserviceaccount.com"` |
| `google_cse_id` | Custom Search Engine ID | Get from [Programmable Search](https://programmablesearchengine.google.com/) |

#### Runtime Configuration

| Parameter | Description | Dev | Prod |
|-----------|-------------|-----|------|
| `memory` | Memory per instance | `"512Mi"` | `"1Gi"` - `"2Gi"` |
| `cpu` | CPU cores (1-4) | `1` | `2` - `4` |
| `timeout` | Request timeout (seconds, max 3600) | `300` | `300` |
| `concurrency` | Max concurrent requests per instance | `80` | `80` |
| `min_instances` | Minimum instances (0 = scale to zero) | `0` | `1+` (for availability) |
| `max_instances` | Maximum instances | `3` | `10+` |

**Cost considerations**:
- `min_instances: 0` in dev saves costs by scaling to zero when idle
- `min_instances: 1+` in prod ensures availability but incurs continuous costs

## Deployment Workflow

### Safety-First: Dry-Run Before Deploy

**Always preview deployment changes before executing**:

```bash
# Preview development deployment
make deploy-dev-dry

# Review the deployment plan, then execute if satisfied
make deploy-dev
```

### Development Deployment

```bash
# 1. Preview changes
make deploy-dev-dry

# 2. Review output:
#    - Docker image tag
#    - Environment variables
#    - Resource configuration
#    - Region and service name

# 3. Execute deployment
make deploy-dev
```

**What happens during deployment**:

1. **Build deployment tool** (if not already built)
2. **Load configuration** from `config/dev.yaml` and `config/.env.dev`
3. **Validate environment** (check GCP credentials, project access)
4. **Build Docker image** (`linux/amd64` platform)
5. **Push image to GCR** (`gcr.io/{project}/{service}:latest`)
6. **Deploy to Cloud Run** with configured resources
7. **Enable public access** (IAM policy: `allUsers → roles/run.invoker`)
8. **Verify traffic routing** (polls until 100% traffic reaches new revision)
9. **Tag deployment** (non-fatal, for tracking)
10. **Run validation** (QA harness health checks)

### Production Deployment

```bash
# Preview production deployment
make deploy-prod-dry

# Carefully review output, then execute
make deploy-prod
```

### First-Time Deployment Notes

On your first deployment to an environment, you may need to:

1. **Grant Cloud Run permissions** to your user:
   ```bash
   gcloud projects add-iam-policy-binding your-project-id \
     --member="user:your-email@example.com" \
     --role="roles/run.admin"
   ```

2. **Grant Artifact Registry permissions** (if using Artifact Registry instead of GCR):
   ```bash
   gcloud projects add-iam-policy-binding your-project-id \
     --member="user:your-email@example.com" \
     --role="roles/artifactregistry.writer"
   ```

## Traffic Routing Verification

### Why This Matters

Cloud Run exhibits **eventual consistency** in traffic routing. After deploying a new revision, it takes 30-60 seconds for traffic to fully migrate.

**Without verification**, post-deployment health checks might hit the old revision, causing false negatives.

### How It Works

The deployment system automatically verifies traffic routing using **exponential backoff polling**:

- **Initial delay**: 1 second
- **Max delay**: 30 seconds
- **Max attempts**: 10 (up to 5 minutes total)
- **Backoff multiplier**: 2.0x (1s → 2s → 4s → 8s → 16s → 30s cap)

The system polls Cloud Run's API to check:
1. Latest ready revision name
2. Traffic allocation percentage

**Success**: 100% of traffic routed to latest revision
**Failure**: Reports actual traffic percentage received

### Manual Verification

If you want to manually verify traffic routing:

```bash
# Get service URL
SERVICE_URL=$(gcloud run services describe arguseek-dev \
  --region=us-central1 \
  --format='value(status.url)')

# Check which revision is serving traffic
curl $SERVICE_URL/health

# View traffic allocation
gcloud run services describe arguseek-dev \
  --region=us-central1 \
  --format='yaml(status.traffic)'
```

## Rollback

The deployment system includes built-in rollback functionality.

### Interactive Rollback

```bash
# Development
./bin/deploy rollback dev

# Production
./bin/deploy rollback prod
```

This shows a list of recent revisions and lets you select which one to rollback to.

### Rollback to Previous Version

```bash
# Rollback to immediately previous revision
./bin/deploy rollback prod -1

# Rollback 2 versions back
./bin/deploy rollback prod -2
```

### Manual Rollback

Using gcloud directly:

```bash
# List revisions
gcloud run revisions list \
  --service=arguseek-prod \
  --region=us-central1

# Route traffic to specific revision
gcloud run services update-traffic arguseek-prod \
  --region=us-central1 \
  --to-revisions=arguseek-prod-00042=100
```
