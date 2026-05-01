#!/usr/bin/env bash
# SentinelAegis — Cloud Run Deployment Script
# Usage: ./scripts/deploy.sh [PROJECT_ID] [REGION]
set -euo pipefail

PROJECT_ID="${1:-$(gcloud config get-value project 2>/dev/null)}"
REGION="${2:-us-central1}"
SERVICE_NAME="sentinelaegis"

echo "╔════════════════════════════════════════════════╗"
echo "║   SentinelAegis — Cloud Run Deployment        ║"
echo "╠════════════════════════════════════════════════╣"
echo "║  Project: ${PROJECT_ID}"
echo "║  Region:  ${REGION}"
echo "║  Service: ${SERVICE_NAME}"
echo "╚════════════════════════════════════════════════╝"
echo ""

# Step 1: Verify gcloud authentication
echo "🔐 Step 1: Verifying GCP authentication..."
gcloud auth print-access-token > /dev/null 2>&1 || {
    echo "❌ Not authenticated. Run: gcloud auth login"
    exit 1
}
echo "   ✅ Authenticated"

# Step 2: Set project
echo "📋 Step 2: Setting project..."
gcloud config set project "${PROJECT_ID}"
echo "   ✅ Project set to ${PROJECT_ID}"

# Step 3: Enable required APIs
echo "🔧 Step 3: Enabling required APIs..."
gcloud services enable \
    run.googleapis.com \
    artifactregistry.googleapis.com \
    cloudbuild.googleapis.com \
    secretmanager.googleapis.com \
    2>/dev/null
echo "   ✅ APIs enabled"

# Step 4: Verify GEMINI_API_KEY
echo "🔑 Step 4: Verifying GEMINI_API_KEY..."
if [ -z "${GEMINI_API_KEY:-}" ]; then
    echo "   ⚠️  GEMINI_API_KEY not set. Agents will use rule-based fallbacks."
    echo "   Set it: export GEMINI_API_KEY=your_key_here"
    GEMINI_ENV=""
else
    echo "   ✅ GEMINI_API_KEY configured"
    GEMINI_ENV="GEMINI_API_KEY=${GEMINI_API_KEY},"
fi

# Step 5: Build and deploy
echo "🚀 Step 5: Building and deploying to Cloud Run..."
gcloud run deploy "${SERVICE_NAME}" \
    --source . \
    --region "${REGION}" \
    --allow-unauthenticated \
    --min-instances 1 \
    --max-instances 3 \
    --memory 256Mi \
    --cpu 1 \
    --timeout 60s \
    --set-env-vars "${GEMINI_ENV}MODEL_NAME=gemini-1.5-pro"

# Step 6: Get the URL
echo ""
echo "🔗 Step 6: Getting service URL..."
SERVICE_URL=$(gcloud run services describe "${SERVICE_NAME}" \
    --region "${REGION}" \
    --format='value(status.url)')

echo ""
echo "╔════════════════════════════════════════════════╗"
echo "║   ✅ Deployment Complete!                     ║"
echo "╠════════════════════════════════════════════════╣"
echo "║  URL: ${SERVICE_URL}"
echo "║  Health: ${SERVICE_URL}/healthz"
echo "║  Dashboard: ${SERVICE_URL}/"
echo "╚════════════════════════════════════════════════╝"

# Step 7: Verify health
echo ""
echo "🏥 Step 7: Health check..."
curl -s "${SERVICE_URL}/healthz" | python3 -m json.tool 2>/dev/null || \
    curl -s "${SERVICE_URL}/healthz"
echo ""
echo "🎉 Done!"
