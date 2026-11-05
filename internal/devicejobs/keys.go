// Package devicejobs is the QC-2 side of the device bridge's job contract
// (docs/design/device-job-contract.md): signed job, result and heartbeat
// envelopes exchanged with the home agent through a separate jobs bucket.
//
// The contract is shared with a Python agent. testdata/devicejobs/vectors.json
// holds the cross-language test vectors; changing any behaviour they pin is a
// contract change.
package devicejobs

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
)

// KeyHexLen is the length of DEVICE_JOB_KEY: 32 random bytes as hex
// (`openssl rand -hex 32`).
const KeyHexLen = 64

var keyHexRe = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

// ErrBadKey reports a key that is not exactly 64 hex characters.
var ErrBadKey = errors.New("device job key must be exactly 64 hex characters (openssl rand -hex 32)")

// kidTag domain-separates the key id hash from every other use of the key.
const kidTag = "qc2-device-kid/v1\n"

// ParseKey trims surrounding whitespace, requires exactly 64 hex characters,
// and returns the 32 decoded bytes. The HMAC key is these bytes, never the
// ASCII hex string.
func ParseKey(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if !keyHexRe.MatchString(s) {
		return nil, ErrBadKey
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, ErrBadKey
	}
	return b, nil
}

// KID is the key id carried in an envelope: the first 4 bytes of
// SHA-256("qc2-device-kid/v1\n" || key), as 8 lowercase hex characters.
func KID(key []byte) string {
	h := sha256.New()
	h.Write([]byte(kidTag))
	h.Write(key)
	return hex.EncodeToString(h.Sum(nil)[:4])
}

type namedKey struct {
	kid string
	key []byte
}

// Keyring holds DEVICE_JOB_KEY (current) and the optional DEVICE_JOB_KEY_NEXT.
// Seal always signs with the current key; Open accepts either, selected by kid.
// Its String and GoString redact, so a stray %v never prints a key.
type Keyring struct {
	cur  namedKey
	next *namedKey
}

// NewKeyring parses the current key and an optional next key ("" for none).
// A malformed next key is an error rather than silently ignored, so a typo
// during rotation is noticed before the old key is retired.
func NewKeyring(current, next string) (*Keyring, error) {
	ck, err := ParseKey(current)
	if err != nil {
		return nil, errors.New("DEVICE_JOB_KEY: " + err.Error())
	}
	kr := &Keyring{cur: namedKey{kid: KID(ck), key: ck}}
	if strings.TrimSpace(next) != "" {
		nk, err := ParseKey(next)
		if err != nil {
			return nil, errors.New("DEVICE_JOB_KEY_NEXT: " + err.Error())
		}
		if KID(nk) != kr.cur.kid {
			kr.next = &namedKey{kid: KID(nk), key: nk}
		}
	}
	return kr, nil
}

// CurrentKID is the kid of the signing key.
func (k *Keyring) CurrentKID() string { return k.cur.kid }

func (k *Keyring) lookup(kid string) ([]byte, bool) {
	if kid == k.cur.kid {
		return k.cur.key, true
	}
	if k.next != nil && kid == k.next.kid {
		return k.next.key, true
	}
	return nil, false
}

// keys returns every configured key, current first.
func (k *Keyring) keys() [][]byte {
	out := [][]byte{k.cur.key}
	if k.next != nil {
		out = append(out, k.next.key)
	}
	return out
}

func (k *Keyring) String() string   { return "devicejobs.Keyring{[redacted]}" }
func (k *Keyring) GoString() string { return k.String() }
