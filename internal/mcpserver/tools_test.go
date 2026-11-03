package mcpserver

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/weitzer-org/sound-builder/internal/presetsvc"
)

// toolError is the boundary that turns service errors into messages a model can act
// on. Each branch must keep naming the next step, and unknown errors must never leak.
func TestToolError(t *testing.T) {
	const secret = "s3://prod-bucket/presets/abc.json: AccessDenied key=AKIA-SECRET"
	for _, tc := range []struct {
		name    string
		err     error
		want    []string
		notWant []string
	}{
		{"run busy", presetsvc.ErrBusy, []string{"already in progress", "Do not start a second run"}, nil},
		{"preset busy", presetsvc.ErrPresetBusy, []string{"being refined", "try again"}, nil},
		{"wrapped preset busy", fmt.Errorf("saving: %w", presetsvc.ErrPresetBusy), []string{"being refined"}, nil},
		{"lookup", &presetsvc.LookupError{Err: errors.New(secret)}, []string{"list_presets", "does not exist"}, []string{"prod-bucket", "AKIA"}},
		{"credentials", &presetsvc.CredentialsError{Public: "Missing Secure AI Credentials", Err: errors.New(secret)},
			[]string{"Missing Secure AI Credentials", "retrying will not help"}, []string{"prod-bucket", "AKIA"}},
		{"unknown", errors.New(secret), []string{"internal error"}, []string{"prod-bucket", "AKIA", "AccessDenied"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			log.SetOutput(&logs)
			defer log.SetOutput(os.Stderr)

			got := toolError("test_tool", tc.err)
			if got == nil {
				t.Fatal("toolError returned nil")
			}
			for _, w := range tc.want {
				if !strings.Contains(got.Error(), w) {
					t.Errorf("message %q lacks %q", got, w)
				}
			}
			for _, nw := range tc.notWant {
				if strings.Contains(got.Error(), nw) {
					t.Errorf("message %q leaks %q", got, nw)
				}
			}
			// Only the unknown branch logs the raw error (server side); the mapped ones must not.
			if logged := strings.Contains(logs.String(), "AccessDenied"); logged != (tc.name == "unknown") {
				t.Errorf("raw error logged = %v for %q", logged, tc.name)
			}
		})
	}
}
