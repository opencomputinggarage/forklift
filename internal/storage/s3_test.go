package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// fakeS3 is an in-memory s3API for hermetic tests (no network, no localstack).
type fakeS3 struct {
	mu       sync.Mutex
	objects  map[string][]byte
	pageSize int // when > 0, caps ListObjectsV2 page size to exercise pagination
}

func newFakeS3() *fakeS3 { return &fakeS3{objects: map[string][]byte{}} }

func (f *fakeS3) PutObject(ctx context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	b, err := io.ReadAll(in.Body)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[aws.ToString(in.Key)] = b
	return &s3.PutObjectOutput{}, nil
}

func (f *fakeS3) GetObject(ctx context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.objects[aws.ToString(in.Key)]
	if !ok {
		return nil, &types.NoSuchKey{}
	}
	return &s3.GetObjectOutput{
		Body:          io.NopCloser(bytes.NewReader(b)),
		ContentLength: aws.Int64(int64(len(b))),
	}, nil
}

func (f *fakeS3) HeadObject(ctx context.Context, in *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.objects[aws.ToString(in.Key)]
	if !ok {
		return nil, &types.NotFound{}
	}
	return &s3.HeadObjectOutput{ContentLength: aws.Int64(int64(len(b)))}, nil
}

func (f *fakeS3) DeleteObject(ctx context.Context, in *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.objects, aws.ToString(in.Key))
	return &s3.DeleteObjectOutput{}, nil
}

func (f *fakeS3) ListObjectsV2(ctx context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	prefix := aws.ToString(in.Prefix)
	var keys []string
	for k := range f.objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	start := 0
	if in.ContinuationToken != nil {
		start = sort.SearchStrings(keys, aws.ToString(in.ContinuationToken))
	}
	limit := len(keys) - start
	if f.pageSize > 0 && f.pageSize < limit {
		limit = f.pageSize
	}
	if in.MaxKeys != nil && int(*in.MaxKeys) > 0 && int(*in.MaxKeys) < limit {
		limit = int(*in.MaxKeys)
	}
	end := start + limit

	out := &s3.ListObjectsV2Output{}
	if end < len(keys) {
		out.IsTruncated = aws.Bool(true)
		out.NextContinuationToken = aws.String(keys[end])
	}
	for _, k := range keys[start:end] {
		out.Contents = append(out.Contents, types.Object{Key: aws.String(k)})
	}
	return out, nil
}

func newTestS3Store(t *testing.T) *S3BlobStore {
	t.Helper()
	return &S3BlobStore{
		api:     newFakeS3(),
		bucket:  "test-bucket",
		prefix:  "forklift",
		tempDir: t.TempDir(),
	}
}

func TestS3BlobStoreRoundTrip(t *testing.T) {
	s := newTestS3Store(t)
	ctx := context.Background()

	data := []byte("hello forklift over s3")
	want := sha256.Sum256(data)
	wantHex := hex.EncodeToString(want[:])

	digest, n, err := s.Put(ctx, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("put: %v", err)
	}
	if digest != wantHex {
		t.Fatalf("digest = %s, want %s", digest, wantHex)
	}
	if n != int64(len(data)) {
		t.Fatalf("size = %d, want %d", n, len(data))
	}

	ok, err := s.Exists(ctx, digest)
	if err != nil || !ok {
		t.Fatalf("exists = %v, %v", ok, err)
	}

	rc, size, err := s.Open(ctx, digest)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer rc.Close()
	if size != int64(len(data)) {
		t.Fatalf("open size = %d", size)
	}
	got, _ := io.ReadAll(rc)
	if !bytes.Equal(got, data) {
		t.Fatalf("read = %q, want %q", got, data)
	}

	// The temp file used for hashing must not linger.
	entries, _ := os.ReadDir(s.tempDir)
	if len(entries) != 0 {
		t.Fatalf("temp dir not cleaned: %d entries", len(entries))
	}
}

func TestS3BlobStoreDedup(t *testing.T) {
	s := newTestS3Store(t)
	f := s.api.(*fakeS3)
	ctx := context.Background()

	d1, _, err := s.Put(ctx, strings.NewReader("same bytes"))
	if err != nil {
		t.Fatal(err)
	}
	d2, _, err := s.Put(ctx, strings.NewReader("same bytes"))
	if err != nil {
		t.Fatal(err)
	}
	if d1 != d2 {
		t.Fatalf("expected identical digests, got %s and %s", d1, d2)
	}
	if len(f.objects) != 1 {
		t.Fatalf("expected 1 stored object after dedup, got %d", len(f.objects))
	}
}

func TestS3BlobStoreDelete(t *testing.T) {
	s := newTestS3Store(t)
	ctx := context.Background()

	digest, _, _ := s.Put(ctx, strings.NewReader("to delete"))
	if err := s.Delete(ctx, digest); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if ok, _ := s.Exists(ctx, digest); ok {
		t.Fatal("blob should be gone")
	}
	// Deleting a missing blob is a no-op.
	if err := s.Delete(ctx, digest); err != nil {
		t.Fatalf("delete missing: %v", err)
	}
}

func TestS3BlobStoreOpenMissing(t *testing.T) {
	s := newTestS3Store(t)
	ctx := context.Background()

	if _, _, err := s.Open(ctx, strings.Repeat("a", 64)); err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	// Invalid digests are treated as not found, not errors.
	if _, _, err := s.Open(ctx, "short"); err != ErrNotFound {
		t.Fatalf("invalid digest err = %v", err)
	}
	if ok, _ := s.Exists(ctx, "bad"); ok {
		t.Fatal("invalid digest should not exist")
	}
}

func TestS3BlobStoreWalkDigests(t *testing.T) {
	s := newTestS3Store(t)
	f := s.api.(*fakeS3)
	f.pageSize = 2 // force the paginator to fetch multiple pages
	ctx := context.Background()

	want := map[string]bool{}
	for i := range 7 {
		d, _, err := s.Put(ctx, strings.NewReader(fmt.Sprintf("blob-%d", i)))
		if err != nil {
			t.Fatal(err)
		}
		want[d] = true
	}

	var got []string
	if err := s.WalkDigests(ctx, func(d string) error {
		got = append(got, d)
		return nil
	}); err != nil {
		t.Fatalf("walk: %v", err)
	}

	if len(got) != len(want) {
		t.Fatalf("walked %d digests, want %d", len(got), len(want))
	}
	if !sort.StringsAreSorted(got) {
		t.Fatalf("digests not in lexicographic order: %v", got)
	}
	for _, d := range got {
		if !want[d] {
			t.Fatalf("unexpected digest %s", d)
		}
	}
}

func TestS3BlobStoreWalkDigestsFnError(t *testing.T) {
	s := newTestS3Store(t)
	ctx := context.Background()
	for i := range 3 {
		if _, _, err := s.Put(ctx, strings.NewReader(fmt.Sprintf("x-%d", i))); err != nil {
			t.Fatal(err)
		}
	}

	stop := errors.New("stop")
	calls := 0
	err := s.WalkDigests(ctx, func(string) error {
		calls++
		return stop
	})
	if !errors.Is(err, stop) {
		t.Fatalf("err = %v, want stop", err)
	}
	if calls != 1 {
		t.Fatalf("fn called %d times, want 1 (walk should stop on error)", calls)
	}
}

func TestS3BlobStoreWalkDigestsCtxCancel(t *testing.T) {
	s := newTestS3Store(t)
	bg := context.Background()
	for i := range 3 {
		if _, _, err := s.Put(bg, strings.NewReader(fmt.Sprintf("y-%d", i))); err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithCancel(bg)
	cancel()
	if err := s.WalkDigests(ctx, func(string) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
