package devicejobs

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ConfirmTTL is how long a dry run's confirm token stays valid.
const ConfirmTTL = 10 * time.Minute

// confirmSkew tolerates a token minted on another machine with a slightly
// faster clock.
const confirmSkew = time.Minute

var (
	expRe = regexp.MustCompile(`^[1-9][0-9]{0,17}$`)
	macRe = regexp.MustCompile(`^[0-9a-f]{32}$`)
)

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
	// Only the canonical form IssueConfirm writes: ParseInt alone would also
	// take "+exp" and "0exp", and each spelling would derive a different job id.
	if !expRe.MatchString(parts[1]) || !macRe.MatchString(parts[2]) {
		return ErrConfirmInvalid
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return ErrConfirmInvalid
	}
	got, err := hex.DecodeString(parts[2])
	if err != nil {
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

// JobIDFromToken derives the push job's id from its confirm token (only ever
// called on a token CheckConfirm accepted, which is canonical), so a repeated
// confirm names the same job: a UUID (version 8, RFC 9562 variant)
// from SHA-256("qc2-job-id/v1\n" || token).
func JobIDFromToken(token string) string {
	return uuid8(jobIDTag + token)
}

func uuid8(s string) string {
	h := sha256.Sum256([]byte(s))
	b := h[:16]
	b[6] = (b[6] & 0x0f) | 0x80
	b[8] = (b[8] & 0x3f) | 0x80
	x := hex.EncodeToString(b)
	return x[0:8] + "-" + x[8:12] + "-" + x[12:16] + "-" + x[16:20] + "-" + x[20:32]
}

// ---- edit confirm tokens (contract section 12, revision E3) ----
//
// An edit token is "e1.<exp_unix>.<32 hex>", minted under its own derived key and claims tag, so a
// push token is never an edit token and the reverse. It binds what the user saw in the dry run:
// the target preset and guitar, the save mode, the open slot, the copy's slot and name, the
// preset's name on the unit, the read it was planned against, and the plan's hash. Phase E3
// only issues and checks it: edit_device_preset does not call these until E4.

const (
	editConfirmPrefix = "e1"
	editConfirmKeyTag = "qc2-edit-confirm-key/v1"
	editJobIDTag      = "qc2-edit-job-id/v1\n"
)

// EditClaims are what an edit confirm token binds.
type EditClaims struct {
	PresetID   string
	Guitar     string
	Mode       string
	Slot       string // the open slot (the plan's expect.slot)
	TargetSlot string // copy only; "" otherwise
	Name       string // copy only; "" otherwise
	DeviceName string // the preset's name on the unit (the plan's expect.name)
	Snapshot   string // the read job's id
	PlanSHA256 string
}

func editConfirmMAC(key []byte, c EditClaims, exp int64) []byte {
	claims, _ := json.Marshal([]string{editConfirmPrefix, c.PresetID, c.Guitar, c.Mode, c.Slot, c.TargetSlot, c.Name,
		c.DeviceName, c.Snapshot, c.PlanSHA256, strconv.FormatInt(exp, 10)})
	m := hmac.New(sha256.New, derivedKey(key, editConfirmKeyTag))
	m.Write(claims)
	return m.Sum(nil)[:16]
}

func derivedKey(key []byte, tag string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(tag))
	return m.Sum(nil)
}

// IssueEditConfirm mints an edit token valid for ConfirmTTL.
func (k *Keyring) IssueEditConfirm(c EditClaims, now time.Time) (string, time.Time) {
	exp := now.Add(ConfirmTTL).Truncate(time.Second)
	return fmt.Sprintf("%s.%d.%s", editConfirmPrefix, exp.Unix(), hex.EncodeToString(editConfirmMAC(k.cur.key, c, exp.Unix()))), exp
}

// CheckEditConfirm verifies an edit token against the claims under the current or next key, with
// the same rules as CheckConfirm (canonical form only, expiry, no exp further out than a fresh
// token's). A push token is ErrConfirmInvalid here.
func (k *Keyring) CheckEditConfirm(token string, c EditClaims, now time.Time) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != editConfirmPrefix || !expRe.MatchString(parts[1]) || !macRe.MatchString(parts[2]) {
		return ErrConfirmInvalid
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return ErrConfirmInvalid
	}
	got, err := hex.DecodeString(parts[2])
	if err != nil {
		return ErrConfirmInvalid
	}
	ok := 0
	for _, key := range k.keys() {
		if hmac.Equal(editConfirmMAC(key, c, exp), got) {
			ok = 1
		}
	}
	if ok == 0 {
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

// EditJobIDFromToken derives an edit job's id from its (checked, canonical) token under its own
// tag, so an edit and a push never share an id: a UUID (version 8, RFC 9562 variant) from
// SHA-256("qc2-edit-job-id/v1\n" || token).
func EditJobIDFromToken(token string) string {
	return uuid8(editJobIDTag + token)
}
