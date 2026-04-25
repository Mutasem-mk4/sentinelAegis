# Gmail Integration Setup Guide

## Prerequisites
- Google Cloud project with billing enabled
- A Gmail account for monitoring (e.g., `sentinel-demo@gmail.com`)

## Step 1: Enable Gmail API
```bash
gcloud services enable gmail.googleapis.com
```

## Step 2: Create OAuth2 Credentials (Recommended for Hackathon)

> **Note:** Service accounts require Google Workspace domain-wide delegation which is complex. For hackathon demos, use OAuth2 user credentials instead.

1. Go to **APIs & Services → Credentials** in GCP Console
2. Click **Create Credentials → OAuth 2.0 Client ID**
3. Application type: **Desktop app**
4. Download the JSON file

## Step 3: Get Refresh Token
Run a one-time auth flow to get a refresh token:
```bash
# Use Google's OAuth2 playground or a simple Go script
# Store the resulting refresh token securely
```

## Step 4: Create Pub/Sub Topic
```bash
gcloud pubsub topics create gmail-watch
```

## Step 5: Grant Gmail Publish Rights
```bash
gcloud pubsub topics add-iam-policy-binding gmail-watch \
  --member="serviceAccount:gmail-api-push@system.gserviceaccount.com" \
  --role="roles/pubsub.publisher"
```

## Step 6: Create Push Subscription
```bash
gcloud pubsub subscriptions create gmail-watch-sub \
  --topic=gmail-watch \
  --push-endpoint=https://YOUR_CLOUD_RUN_URL/pubsub/push \
  --ack-deadline=10
```

## Step 7: Set Environment Variables
```bash
export GMAIL_CREDENTIALS_JSON=$(base64 -w0 credentials.json)
export GCP_PROJECT_ID=your-project-id
export PUBSUB_TOPIC=gmail-watch
export MONITORED_EMAIL=sentinel-demo@gmail.com
```

## Step 8: Deploy & Test
```bash
gcloud run deploy sentinelaegis --source . --region us-central1 \
  --allow-unauthenticated --min-instances 1 \
  --set-env-vars GEMINI_API_KEY=$GEMINI_API_KEY,GMAIL_CREDENTIALS_JSON=$GMAIL_CREDENTIALS_JSON,GCP_PROJECT_ID=$GCP_PROJECT_ID,MONITORED_EMAIL=$MONITORED_EMAIL
```

Send an email to your monitored inbox and watch the dashboard react within 3-5 seconds.

## Troubleshooting
- **Pub/Sub not reaching Cloud Run:** Ensure Cloud Run URL is public (`--allow-unauthenticated`)
- **Gmail watch() expired:** Restart the app (watch lasts 7 days)
- **SSE drops:** Browser auto-reconnects (built into EventSource API)
- **Gemini rate limit:** System falls back to rule-based analysis automatically
