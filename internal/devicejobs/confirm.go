package devicejobs

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ConfirmTTL is how long a dry run's confirm token stays valid.
const ConfirmTTL = 10 * time.Minute

// confirmSkew tolerates a token minted on another machine with a slightly
// faster clock.
const confirmSkew = time.Minute

const (
	confirmPrefix = "c1"
	confirmKeyTag = "qc2-confirm-key/v1"
	jobIDTag      = "qc2-job-id/v1\n"
)

// ConfirmClaims are what a confirm token binds: the push arguments and the
// exact export the user saw.
type ConfirmClaims struct {
	PresetID   string
	Guitar     string
	Slot       string
	Name       string
	PlanSHA256 string
}

// ErrConfirmExpired and ErrConfirmMismatch are the two ways a well-formed
// token fails; anything else is ErrConfirmInvalid.
var (
	ErrConfirmInvalid  = errors.New("confirm_token is not a token from a dry run of this server")
	ErrConfirmExpired  = errors.New("confirm_token has expired")
	ErrConfirmMismatch = errors.New("confirm_token does not match these arguments or the current preset")
)

func confirmKey(key []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(confirmKeyTag))
	return m.Sum(nil)
}

func confirmMAC(key []byte, c ConfirmClaims, exp int64) []byte {
	// A JSON array, so no field value can shift into another (names may
	// contain any character).
	claims, _ := json.Marshal([]string{confirmPrefix, c.PresetID, c.Guitar, c.Slot, c.Name, c.PlanSHA256, strconv.FormatInt(exp, 10)})
	m := hmac.New(sha256.New, confirmKey(key))
	m.Write(claims)
	return m.Sum(nil)[:16]
}

// IssueConfirm mints a token "c1.<exp_unix>.<32 hex>" valid for ConfirmTTL.
func (k *Keyring) IssueConfirm(c ConfirmClaims, now time.Time) (string, time.Time) {
	exp := now.Add(ConfirmTTL).Truncate(time.Second)
	return fmt.Sprintf("%s.%d.%s", confirmPrefix, exp.Unix(), hex.EncodeToString(confirmMAC(k.cur.key, c, exp.Unix()))), exp
}

// CheckConfirm verifies token against the claims under the current or next
// key (so a dry run made just before a key swap still confirms).
func (k *Keyring) CheckConfirm(token string, c ConfirmClaims, now time.Time) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != confirmPrefix || len(parts[2]) != 32 {
		return ErrConfirmInvalid
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || exp <= 0 {
		return ErrConfirmInvalid
	}
	got, err := hex.DecodeString(parts[2])
	if err != nil || strings.ToLower(parts[2]) != parts[2] {
		return ErrConfirmInvalid
	}
	ok := 0
	for _, key := range k.keys() {
		if hmac.Equal(confirmMAC(key, c, exp), got) {
			ok = 1
		}
	}
	if ok == 0 {
		// Either forged or the claims differ; a real token for other
		// arguments is the likely case, so say that.
		return ErrConfirmMismatch
	}
	expT := time.Unix(exp, 0)
	if !now.Before(expT) {
		return ErrConfirmExpired
	}
	if expT.After(now.Add(ConfirmTTL + confirmSkew)) {
		return ErrConfirmInvalid
	}
	return nil
}

// JobIDFromToken derives the push job's id from its confirm token, so a
// repeated confirm names the same job: a UUID (version 8, RFC 9562 variant)
// from SHA-256("qc2-job-id/v1\n" || token).
func JobIDFromToken(token string) string {
	h := sha256.Sum256([]byte(jobIDTag + token))
	b := h[:16]
	b[6] = (b[6] & 0x0f) | 0x80
	b[8] = (b[8] & 0x3f) | 0x80
	x := hex.EncodeToString(b)
	return x[0:8] + "-" + x[8:12] + "-" + x[12:16] + "-" + x[16:20] + "-" + x[20:32]
}
