package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// s3API is the subset of the S3 client used by the storage and objstore
// packages. Narrowing it to these methods lets both be unit-tested against an
// in-memory fake with no network. *s3.Client satisfies it.
type s3API interface {
	GetObject(ctx context.Context, in *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	HeadObject(ctx context.Context, in *s3.HeadObjectInput, opts ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	PutObject(ctx context.Context, in *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	DeleteObject(ctx context.Context, in *s3.DeleteObjectInput, opts ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	ListObjectsV2(ctx context.Context, in *s3.ListObjectsV2Input, opts ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

// S3API is the exported alias used by other packages (e.g. objstore) that share
// the same client and fake.
type S3API = s3API

// S3Config configures the S3 client. Region/Endpoint/credentials are optional:
// an empty Region/credentials falls back to the AWS default chain, which covers
// EKS IRSA and EKS Pod Identity automatically.
type S3Config struct {
	Bucket          string
	Prefix          string
	Region          string
	Endpoint        string // custom endpoint for MinIO/localstack; empty uses AWS
	ForcePathStyle  bool   // true for MinIO-style path addressing
	AccessKeyID     string // optional; empty uses the default credential chain
	SecretAccessKey string
}

// NewS3Client builds an *s3.Client from cfg. The AWS SDK v2 default credential
// chain resolves IRSA (web identity) and EKS Pod Identity (container
// credentials) with no extra code; static keys are used only when both are set.
func NewS3Client(ctx context.Context, cfg S3Config) (*s3.Client, error) {
	var opts []func(*awsconfig.LoadOptions) error
	if cfg.Region != "" {
		opts = append(opts, awsconfig.WithRegion(cfg.Region))
	}
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.ForcePathStyle
	}), nil
}

// S3BlobStore is an S3-backed BlobStore. Blobs are immutable and
// content-addressed, so every replica can read and write the same bucket
// concurrently without coordination — this is what lets an HA deployment share
// blobs without a ReadWriteMany volume. Objects are laid out as
// <prefix>/blobs/aa/bb/<digest>, mirroring FSStore so WalkDigests yields the
// same lexicographic (== digest) order.
//
// S3 has been strongly read-after-write and list consistent since Dec 2020, so
// no read-retry workarounds are needed.
type S3BlobStore struct {
	api     s3API
	client  *s3.Client
	bucket  string
	prefix  string // normalized: no leading/trailing slash, may be ""
	tempDir string // local dir for hashing during Put
}

// NewS3BlobStore builds an S3-backed blob store. tempDir holds the transient
// file each Put streams through while hashing (removed immediately after
// upload); in s3 mode this lives on the pod's emptyDir.
func NewS3BlobStore(ctx context.Context, cfg S3Config, tempDir string) (*S3BlobStore, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 blob store requires a bucket")
	}
	client, err := NewS3Client(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return nil, fmt.Errorf("create blob temp dir: %w", err)
	}
	return &S3BlobStore{
		api:     client,
		client:  client,
		bucket:  cfg.Bucket,
		prefix:  strings.Trim(cfg.Prefix, "/"),
		tempDir: tempDir,
	}, nil
}

// Client returns the underlying S3 client so co-located components (e.g. the
// metadata sync) can share one configured client and bucket.
func (s *S3BlobStore) Client() *s3.Client { return s.client }

func (s *S3BlobStore) key(digest string) string {
	// Fan out by the first two byte-pairs, matching FSStore's on-disk layout.
	return path.Join(s.prefix, "blobs", digest[0:2], digest[2:4], digest)
}

func (s *S3BlobStore) blobsPrefix() string {
	return path.Join(s.prefix, "blobs") + "/"
}

// Put implements BlobStore. It streams r into a local temp file while hashing,
// then uploads to the digest key. Because the digest (and therefore the key) is
// known only after the full stream is read, the temp file lets the upload target
// the final key directly — no temp S3 key or server-side copy. Re-putting an
// existing blob skips the upload.
func (s *S3BlobStore) Put(ctx context.Context, r io.Reader) (string, int64, error) {
	tmp, err := os.CreateTemp(s.tempDir, "blob-*")
	if err != nil {
		return "", 0, fmt.Errorf("create temp blob: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		os.Remove(tmpName)
	}()

	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), r)
	if err != nil {
		return "", 0, fmt.Errorf("buffer blob: %w", err)
	}
	digest := hex.EncodeToString(h.Sum(nil))
	key := s.key(digest)

	// Identical bytes by construction; skip the upload if already present.
	exists, err := s.existsKey(ctx, key)
	if err != nil {
		return "", 0, err
	}
	if exists {
		return digest, n, nil
	}

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return "", 0, fmt.Errorf("rewind temp blob: %w", err)
	}
	// The temp file is seekable, so the SDK sets Content-Length and the payload
	// hash without buffering the blob in memory. Single PutObject caps at 5 GiB,
	// which is far above any real package artifact.
	if _, err := s.api.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          tmp,
		ContentLength: aws.Int64(n),
	}); err != nil {
		return "", 0, fmt.Errorf("upload blob: %w", err)
	}
	return digest, n, nil
}

// Open implements BlobStore.
func (s *S3BlobStore) Open(ctx context.Context, digest string) (io.ReadCloser, int64, error) {
	if !validDigest(digest) {
		return nil, 0, ErrNotFound
	}
	out, err := s.api.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(digest)),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, 0, ErrNotFound
		}
		return nil, 0, err
	}
	return out.Body, aws.ToInt64(out.ContentLength), nil
}

// Exists implements BlobStore.
func (s *S3BlobStore) Exists(ctx context.Context, digest string) (bool, error) {
	if !validDigest(digest) {
		return false, nil
	}
	return s.existsKey(ctx, s.key(digest))
}

func (s *S3BlobStore) existsKey(ctx context.Context, key string) (bool, error) {
	_, err := s.api.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		return true, nil
	}
	if isNotFound(err) {
		return false, nil
	}
	return false, err
}

// Delete implements BlobStore. Deleting a missing blob is a no-op.
func (s *S3BlobStore) Delete(ctx context.Context, digest string) error {
	if !validDigest(digest) {
		return nil
	}
	_, err := s.api.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key(digest)),
	})
	if err != nil && !isNotFound(err) {
		return err
	}
	return nil
}

// WalkDigests implements WalkableStore. S3 returns keys in lexicographic order,
// which equals digest order given the layout, so replication's streaming merge
// works unchanged. Walking stops at the first error fn returns.
func (s *S3BlobStore) WalkDigests(ctx context.Context, fn func(digest string) error) error {
	p := s3.NewListObjectsV2Paginator(s.api, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(s.blobsPrefix()),
	})
	for p.HasMorePages() {
		if err := ctx.Err(); err != nil {
			return err
		}
		page, err := p.NextPage(ctx)
		if err != nil {
			return err
		}
		for _, obj := range page.Contents {
			if err := ctx.Err(); err != nil {
				return err
			}
			d := path.Base(aws.ToString(obj.Key))
			if !validDigest(d) {
				continue
			}
			if err := fn(d); err != nil {
				return err
			}
		}
	}
	return nil
}

// IsNotFound reports whether err is an S3 "no such key"/404. Exported so
// co-located components (e.g. objstore) can treat a missing object as absence
// rather than failure.
func IsNotFound(err error) bool { return isNotFound(err) }

// IsPreconditionFailed reports whether err is an S3 412 Precondition Failed,
// returned when a conditional write (If-Match / If-None-Match) is rejected
// because another writer changed the object first. Co-located components (e.g.
// objstore fencing) treat it as "lost the race, skip this cycle".
func IsPreconditionFailed(err error) bool {
	if re, ok := errors.AsType[*smithyhttp.ResponseError](err); ok {
		return re.HTTPStatusCode() == http.StatusPreconditionFailed
	}
	return false
}

// isNotFound reports whether err is an S3 "no such key"/404, across the SDK's
// typed errors and the generic HTTP response error.
func isNotFound(err error) bool {
	if _, ok := errors.AsType[*types.NoSuchKey](err); ok {
		return true
	}
	if _, ok := errors.AsType[*types.NotFound](err); ok {
		return true
	}
	if re, ok := errors.AsType[*smithyhttp.ResponseError](err); ok {
		return re.HTTPStatusCode() == http.StatusNotFound
	}
	return false
}
