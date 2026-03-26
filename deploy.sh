#!/bin/bash

echo "🚀 Submitting Cloud Build to Private Worker Pool (us-central1)..."
if gcloud beta builds submit --region=us-central1 --config cloudbuild.yaml .; then
    echo ""
    echo "✅ Build Completed Successfully!"
    echo -n "🌐 YOUR LIVE DASHBOARD URL IS: "
    gcloud run services describe sound-builder-dev --region us-central1 --format="value(status.url)"
    echo ""
else
    echo ""
    echo "❌ Build Failed! Attempting to fetch unit test logs from Cloud Storage..."
    gcloud storage cp gs://weitzer-sound-builder/test-results.txt ./test-results.txt || true
    if [ -f "test-results.txt" ]; then
        echo "----------------------------------------------------"
        echo "                 UNIT TEST RESULTS                  "
        echo "----------------------------------------------------"
        cat test-results.txt
        echo "----------------------------------------------------"
    else
        echo "⚠️ No test-results.txt found in GCS. Build likely failed before tests could execute."
    fi
    exit 1
fi
