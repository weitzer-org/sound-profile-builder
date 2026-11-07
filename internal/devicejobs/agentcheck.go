package devicejobs

import "time"

// CheckPendingJob is the reference implementation of the agent's checks on a
// pending job object (contract §8, steps 1-9), in the contract's order. The
// Python agent must reach the same Outcome for every vector in
// testdata/devicejobs/vectors.json; QC-2's own tests use it as a fake agent.
//
// seen reports whether the agent has already taken this job id (its local
// ledger, or an existing jobs/done/<id>.json). A nil seen means none.
//
// On OutcomeExpired the parsed job is returned with the error, because the
// agent writes a signed "expired" result for it; on every other failure the
// job is nil and the agent writes no result.
func CheckPendingJob(kr *Keyring, objectKey, agentBucket string, now time.Time, data []byte, seen func(jobID string) bool) (*Job, error) {
	// 1. Object key.
	m := pendingRe.FindStringSubmatch(objectKey)
	if m == nil {
		return nil, fail(OutcomeBadKey, "not jobs/pending/<lowercase uuid>.json")
	}
	keyID := m[1]
	// 2. Size.
	if len(data) > MaxJobBytes {
		return nil, fail(OutcomeTooLarge, "%d bytes", len(data))
	}
	// 3-5. Envelope, kid, MAC under either job tag; 6. strict payload of the matching version.
	j, err := kr.OpenJob(data)
	if err != nil {
		return nil, err
	}
	// 7. Bindings.
	if j.JobID != keyID {
		return nil, fail(OutcomeMisrouted, "job_id does not match the object key")
	}
	if j.Bucket != agentBucket {
		return nil, fail(OutcomeMisrouted, "job is for another bucket")
	}
	// 8. Replay.
	if seen != nil && seen(j.JobID) {
		return nil, fail(OutcomeReplay, "job already taken")
	}
	// 9. Time.
	created, expires := j.Times()
	switch {
	case !created.Before(expires):
		return nil, fail(OutcomeBadPayload, "created_at must be before expires_at")
	case expires.Sub(created) > MaxJobLifetime:
		return nil, fail(OutcomeBadPayload, "lifetime over 24h")
	case created.After(now.Add(MaxCreatedAhead)):
		return nil, fail(OutcomeBadPayload, "created_at is more than 5 minutes in the future")
	case !now.Before(expires):
		return j, fail(OutcomeExpired, "")
	}
	return j, nil
}
