# Check for bucket arguments
RESULTS_BUCKET=${1:-"REPLACE_WITH_PROD_BUCKET_NAME"}
DATA_BUCKET=${2:-"REPLACE_WITH_PROD_DATA_BUCKET_NAME"}

echo "🚀 Submitting Cloud Build to Private Worker Pool (us-central1)..."
echo "📂 Using Test Results Bucket: $RESULTS_BUCKET"
echo "📂 Using App Data Bucket:     $DATA_BUCKET"

if gcloud beta builds submit --project="quacktastic-waffle" --region=us-central1 --config cloudbuild.yaml --substitutions=_RESULTS_BUCKET=$RESULTS_BUCKET,_DATA_BUCKET=$DATA_BUCKET .; then
    echo ""
    echo "✅ Build Completed Successfully!"
    echo -n "🌐 YOUR LIVE DASHBOARD URL IS: "
    gcloud run services describe sound-builder-dev --project="quacktastic-waffle" --region us-central1 --format="value(status.url)"
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
