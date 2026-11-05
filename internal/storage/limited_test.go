package storage

import (
	"bytes"
	"errors"
	"testing"
)

func TestReadCappedBoundaries(t *testing.T) {
	payload := []byte("0123456789")
	for _, c := range []struct {
		max    int64
		tooBig bool
	}{{15, false}, {10, false}, {9, true}, {0, true}} {
		data, err := readCapped(bytes.NewReader(payload), c.max)
		if c.tooBig {
			if !errors.Is(err, ErrTooLarge) || data != nil {
				t.Errorf("max %d: got %q, %v; want ErrTooLarge", c.max, data, err)
			}
			continue
		}
		if err != nil || !bytes.Equal(data, payload) {
			t.Errorf("max %d: got %q, %v", c.max, data, err)
		}
	}
}
