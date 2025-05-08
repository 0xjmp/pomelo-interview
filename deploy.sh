#!/bin/bash
set -e  

command -v gcloud >/dev/null 2>&1 || { echo "gcloud is required but not installed. Aborting." >&2; exit 1; }
command -v docker >/dev/null 2>&1 || { echo "docker is required but not installed. Aborting." >&2; exit 1; }
command -v terraform >/dev/null 2>&1 || { echo "terraform is required but not installed. Aborting." >&2; exit 1; }

PROJECT_ID="pomelo-459223"
REGION="us-west1"
REGISTRY="${REGION}-docker.pkg.dev"
REPOSITORY="pomelo"

if ! gcloud auth list --filter=status:ACTIVE --format="get(account)" | grep -q "@"; then
    echo "🔑 Not authenticated with Google Cloud. Authenticating..."
    gcloud auth login
else
    echo "✅ Already authenticated with Google Cloud"
fi

CURRENT_PROJECT=$(gcloud config get-value project)
if [ "$CURRENT_PROJECT" != "$PROJECT_ID" ]; then
    echo "🔧 Setting project to ${PROJECT_ID}..."
    gcloud config set project ${PROJECT_ID}
else
    echo "✅ Project already set to ${PROJECT_ID}"
fi

echo "🔧 Configuring Docker for Artifact Registry..."
gcloud auth configure-docker ${REGISTRY}

echo "📦 Creating Artifact Registry repository..."
terraform -chdir=terraform init
terraform -chdir=terraform apply -target=google_artifact_registry_repository.pomelo -auto-approve

echo "🏗️ Building and pushing Docker images..."
docker buildx build --platform linux/amd64 -t ${REGISTRY}/${PROJECT_ID}/${REPOSITORY}/pomelo-web:latest .
docker push ${REGISTRY}/${PROJECT_ID}/${REPOSITORY}/pomelo-web:latest

# Build and push ModSecurity proxy
docker buildx build --platform linux/amd64 -t ${REGISTRY}/${PROJECT_ID}/${REPOSITORY}/modsecurity:latest -f Dockerfile.modsecurity .
docker push ${REGISTRY}/${PROJECT_ID}/${REPOSITORY}/modsecurity:latest

echo "🚀 Deploying infrastructure with Terraform..."
terraform -chdir=terraform apply -auto-approve

echo "✅ Deployment complete!"
echo "Your services should be available at:"
echo "Web service: $(terraform -chdir=terraform output -raw pomelo_web_url)"
echo "ModSecurity proxy: $(terraform -chdir=terraform output -raw pomelo_web_secure_url)" 