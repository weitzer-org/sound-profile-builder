package storage

import (
	"context"
	"strings"
	"testing"
)

func TestGCSAgentLogging(t *testing.T) {
	ctx := context.Background()

	// 1. Initialize the Storage Client
	gcs, err := NewGCSClient(ctx)
	if err != nil {
		t.Skipf("Skipping GCS integration test because credentials are not natively injected in this CI runner: %v", err)
	}
	defer gcs.Close()

	// 2. Define a fake agent payload simulation
	bucket := "weitzer-sound-builder"
	objectName := "logs/test/unit_test_agent_12.json"
	payload := `{"final_html_payload": "test", "agent_impact": ["unit test logging successful"]}`

	// 3. Write payload to the bucket
	err = gcs.WriteFile(ctx, bucket, objectName, []byte(payload))
	if err != nil {
		t.Fatalf("Failed to write agent log to GCS bucket %s: %v", bucket, err)
	}

	// 4. Verify the agent log actually made it up there by reading it back down
	readBytes, err := gcs.ReadFile(ctx, bucket, objectName)
	if err != nil {
		t.Fatalf("Failed to read back the test agent log from GCS: %v", err)
	}

	if !strings.Contains(string(readBytes), "unit test logging successful") {
		t.Errorf("Agent payload saved to Cloud Storage missing expected data. Got: %s", string(readBytes))
	}
}
