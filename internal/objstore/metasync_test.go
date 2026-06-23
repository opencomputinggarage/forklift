package objstore

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/younsl/o/box/kubernetes/forklift/internal/meta"
)

// fakeMetaS3 is a minimal in-memory storage.S3API for MetaSync tests.
type fakeMetaS3 struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newFakeMetaS3() *fakeMetaS3 { return &fakeMetaS3{objects: map[string][]byte{}} }

func (f *fakeMetaS3) PutObject(ctx context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	b, err := io.ReadAll(in.Body)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[aws.ToString(in.Key)] = b
	return &s3.PutObjectOutput{}, nil
}

func (f *fakeMetaS3) GetObject(ctx context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
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

func (f *fakeMetaS3) HeadObject(ctx context.Context, in *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.objects[aws.ToString(in.Key)]; !ok {
		return nil, &types.NotFound{}
	}
	return &s3.HeadObjectOutput{}, nil
}

func (f *fakeMetaS3) DeleteObject(ctx context.Context, in *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.objects, aws.ToString(in.Key))
	return &s3.DeleteObjectOutput{}, nil
}

func (f *fakeMetaS3) ListObjectsV2(ctx context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	return &s3.ListObjectsV2Output{}, nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newMetaSync(t *testing.T, store *meta.Store, fake *fakeMetaS3, dir string) *MetaSync {
	t.Helper()
	return NewMetaSync(MetaOptions{
		Store:    store,
		API:      fake,
		Bucket:   "test-bucket",
		Key:      "forklift/meta/forklift.db",
		DataDir:  dir,
		Interval: time.Minute,
		Log:      testLogger(),
	})
}

// TestMetaSyncRoundTrip uploads a leader snapshot and restores it onto a fresh
// pod, asserting the database content (via PRAGMA user_version) survives the
// round trip through S3.
func TestMetaSyncRoundTrip(t *testing.T) {
	ctx := context.Background()
	fake := newFakeMetaS3()

	// Leader writes a marker and uploads a snapshot.
	dir1 := t.TempDir()
	s1, err := meta.Open(ctx, filepath.Join(dir1, "forklift.db"))
	if err != nil {
		t.Fatalf("open store1: %v", err)
	}
	if _, err := s1.DB().ExecContext(ctx, "PRAGMA user_version = 42"); err != nil {
		t.Fatalf("set marker: %v", err)
	}
	m1 := newMetaSync(t, s1, fake, dir1)
	if err := m1.Promote(ctx); err != nil {
		t.Fatalf("promote: %v", err)
	}
	if err := m1.sync(ctx); err != nil {
		t.Fatalf("leader sync (upload): %v", err)
	}
	s1.Close()

	if _, ok := fake.objects["forklift/meta/forklift.db"]; !ok {
		t.Fatal("snapshot not uploaded to expected key")
	}

	// A fresh pod restores from S3 on boot.
	dir2 := t.TempDir()
	s2, err := meta.Open(ctx, filepath.Join(dir2, "forklift.db"))
	if err != nil {
		t.Fatalf("open store2: %v", err)
	}
	defer s2.Close()
	m2 := newMetaSync(t, s2, fake, dir2)
	if err := m2.RestoreOnBoot(ctx); err != nil {
		t.Fatalf("restore on boot: %v", err)
	}

	var v int
	if err := s2.DB().QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if v != 42 {
		t.Fatalf("user_version = %d, want 42 (restore did not apply snapshot)", v)
	}
}

// TestMetaSyncRestoreEmptyBucket verifies a fresh bucket is a clean no-op.
func TestMetaSyncRestoreEmptyBucket(t *testing.T) {
	ctx := context.Background()
	fake := newFakeMetaS3()
	dir := t.TempDir()
	s, err := meta.Open(ctx, filepath.Join(dir, "forklift.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()
	m := newMetaSync(t, s, fake, dir)
	if err := m.RestoreOnBoot(ctx); err != nil {
		t.Fatalf("restore on empty bucket should be a no-op, got: %v", err)
	}
}

// TestMetaSyncStandbyDownload verifies a standby records the latest snapshot and
// Promote applies it.
func TestMetaSyncStandbyDownload(t *testing.T) {
	ctx := context.Background()
	fake := newFakeMetaS3()

	// Leader uploads.
	dir1 := t.TempDir()
	s1, _ := meta.Open(ctx, filepath.Join(dir1, "forklift.db"))
	if _, err := s1.DB().ExecContext(ctx, "PRAGMA user_version = 7"); err != nil {
		t.Fatal(err)
	}
	m1 := newMetaSync(t, s1, fake, dir1)
	if err := m1.Promote(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m1.sync(ctx); err != nil {
		t.Fatal(err)
	}
	s1.Close()

	// Standby downloads (not leader), then is promoted.
	dir2 := t.TempDir()
	s2, _ := meta.Open(ctx, filepath.Join(dir2, "forklift.db"))
	defer s2.Close()
	m2 := newMetaSync(t, s2, fake, dir2)
	if err := m2.sync(ctx); err != nil { // standby branch: download
		t.Fatalf("standby sync: %v", err)
	}
	if m2.snapshotPath == "" {
		t.Fatal("standby did not record a downloaded snapshot")
	}
	if err := m2.Promote(ctx); err != nil {
		t.Fatalf("promote: %v", err)
	}
	var v int
	if err := s2.DB().QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != 7 {
		t.Fatalf("user_version = %d, want 7", v)
	}
}
