package devicejobs

import (
	"errors"
	"os"
	"strings"
)

// Config enables the device tools: the jobs bucket and the signing keys.
type Config struct {
	Bucket string
	Keys   *Keyring
}

// String redacts the keys.
func (c Config) String() string   { return "devicejobs.Config{Bucket:" + c.Bucket + " Keys:[redacted]}" }
func (c Config) GoString() string { return c.String() }

// ConfigFromEnv reads DEVICE_JOBS_BUCKET, DEVICE_JOB_KEY and
// DEVICE_JOB_KEY_NEXT.
//
//   - Neither the bucket nor the key set: (nil, nil), the feature is off.
//   - Anything set but unusable: (nil, err). The caller logs it and runs
//     without device tools; a device misconfiguration never stops the app.
//
// mainBucket is QC-2's own bucket. The jobs bucket must differ: keeping the
// agent's credentials away from presets, run logs and usage data is the
// premise of the whole design (plan §2).
func ConfigFromEnv(mainBucket string) (*Config, error) {
	return configFrom(os.Getenv, mainBucket)
}

func configFrom(getenv func(string) string, mainBucket string) (*Config, error) {
	bucket := strings.TrimSpace(getenv("DEVICE_JOBS_BUCKET"))
	key := getenv("DEVICE_JOB_KEY")
	next := getenv("DEVICE_JOB_KEY_NEXT")
	if bucket == "" && strings.TrimSpace(key) == "" {
		if strings.TrimSpace(next) != "" {
			return nil, errors.New("DEVICE_JOB_KEY_NEXT is set without DEVICE_JOB_KEY and DEVICE_JOBS_BUCKET")
		}
		return nil, nil
	}
	if bucket == "" {
		return nil, errors.New("DEVICE_JOB_KEY is set but DEVICE_JOBS_BUCKET is not")
	}
	if !ValidBucketName(bucket) {
		return nil, errors.New("DEVICE_JOBS_BUCKET is not a valid bucket name")
	}
	if strings.EqualFold(bucket, strings.TrimSpace(mainBucket)) {
		return nil, errors.New("DEVICE_JOBS_BUCKET must be a separate bucket, not the app's main bucket")
	}
	kr, err := NewKeyring(key, next)
	if err != nil {
		return nil, err
	}
	return &Config{Bucket: bucket, Keys: kr}, nil
}
