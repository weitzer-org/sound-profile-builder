package storage

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Both production backends implement LimitedReader; this fails to compile if
// either stops doing so.
var (
	_ LimitedReader = (*S3Client)(nil)
	_ LimitedReader = (*GCSClient)(nil)
)

// ErrTooLarge is returned by ReadFileLimited when the object exceeds the limit.
var ErrTooLarge = errors.New("object exceeds the size limit")

// LimitedReader is implemented by clients that can read an object without
// downloading more than a limit. It is optional, checked with a type
// assertion, so the Client interface (and its other implementations) stay
// unchanged. Readers of objects written by less-trusted parties (the device
// jobs bucket) use it so a huge object can't exhaust the server's memory.
type LimitedReader interface {
	// ReadFileLimited returns the object if it is at most max bytes, and
	// ErrTooLarge (wrapped) if it is larger. It transfers at most max+1 bytes.
	ReadFileLimited(ctx context.Context, bucket, object string, max int64) ([]byte, error)
}

func readCapped(r io.Reader, max int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("%w (%d bytes)", ErrTooLarge, max)
	}
	return data, nil
}

// ReadFileLimited asks for bytes 0..max only (an HTTP Range request), so at
// most max+1 bytes cross the wire, and caps the read as well.
func (s *S3Client) ReadFileLimited(ctx context.Context, bucket, object string, max int64) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(object),
		Range:  aws.String(fmt.Sprintf("bytes=0-%d", max)),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	return readCapped(out.Body, max)
}

// ReadFileLimited reads at most max+1 bytes through a range reader.
func (g *GCSClient) ReadFileLimited(ctx context.Context, bucket, object string, max int64) ([]byte, error) {
	reader, err := g.client.Bucket(bucket).Object(object).NewRangeReader(ctx, 0, max+1)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return readCapped(reader, max)
}
