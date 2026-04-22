package service

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"proply/internal/config"
)

const (
	maxLogoBytes      = 2 * 1024 * 1024 // 2 MB — logo and team_member
	maxCaseStudyBytes = 5 * 1024 * 1024 // 5 MB — case_study images
	presignTTL        = 15 * time.Minute
)

// allowedContentTypes maps MIME type → file extension for accepted uploads.
var allowedContentTypes = map[string]string{
	"image/png":  "png",
	"image/jpeg": "jpg",
	"image/webp": "webp",
}

// StorageService generates presigned PUT URLs for Hetzner Object Storage (S3-compatible).
type StorageService struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
	publicBase    string // public read base URL: {endpoint}/{bucket}
}

// NewStorageService creates a StorageService. Returns a no-op stub when S3 credentials
// are absent so the rest of the application boots without storage configured.
func NewStorageService(cfg *config.Config) *StorageService {
	if cfg.S3AccessKey == "" || cfg.S3SecretKey == "" || cfg.S3Endpoint == "" {
		return &StorageService{} // presignClient == nil → all calls return ErrStorageNotConfigured
	}

	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(cfg.S3Endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		Region:       cfg.S3Region,
		UsePathStyle: true, // required for S3-compatible endpoints
	})

	return &StorageService{
		client:        client,
		presignClient: s3.NewPresignClient(client),
		bucket:        cfg.S3Bucket,
		publicBase:    fmt.Sprintf("%s/%s", cfg.S3Endpoint, cfg.S3Bucket),
	}
}

// PresignUploadInput carries validated parameters for a presign request.
type PresignUploadInput struct {
	// UserID is required for "logo"; ProposalID + BlockID for "case_study" / "team_member".
	UserID      string
	ProposalID  string
	BlockID     string
	FileType    string // "logo" | "case_study" | "team_member"
	ContentType string // "image/png" | "image/jpeg" | "image/webp"
	SizeBytes   int
}

// PresignResult holds the presigned upload URL and the future public file URL.
type PresignResult struct {
	UploadURL string `json:"upload_url"`
	FileURL   string `json:"file_url"`
}

// PresignUpload validates the request and returns a short-lived PUT URL for direct browser upload.
func (s *StorageService) PresignUpload(ctx context.Context, in PresignUploadInput) (*PresignResult, error) {
	if s.presignClient == nil {
		return nil, ErrStorageNotConfigured
	}

	ext, ok := allowedContentTypes[in.ContentType]
	if !ok {
		return nil, ErrInvalidContentType
	}

	maxSize := maxLogoBytes
	if in.FileType == "case_study" {
		maxSize = maxCaseStudyBytes
	}
	if in.SizeBytes > maxSize {
		return nil, ErrFileTooLarge
	}

	key, err := buildS3Key(in.FileType, in.UserID, in.ProposalID, in.BlockID, ext)
	if err != nil {
		return nil, err
	}

	req, err := s.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(in.ContentType),
		ContentLength: aws.Int64(int64(in.SizeBytes)),
	}, s3.WithPresignExpires(presignTTL))
	if err != nil {
		return nil, fmt.Errorf("storage: presign put: %w", err)
	}

	return &PresignResult{
		UploadURL: req.URL,
		FileURL:   fmt.Sprintf("%s/%s", s.publicBase, key),
	}, nil
}

// DeleteUserObjects removes all S3 objects belonging to a user:
//   - users/{userID}/* (logo)
//   - proposals/{proposalID}/* for every proposal in proposalIDs
//
// Best-effort: errors are logged but do not abort the loop.
func (s *StorageService) DeleteUserObjects(ctx context.Context, userID string, proposalIDs []string) error {
	if s.client == nil {
		return nil // no storage configured — nothing to delete
	}

	// Collect all keys via ListObjectsV2
	var toDelete []types.ObjectIdentifier

	prefixes := []string{fmt.Sprintf("users/%s/", userID)}
	for _, pid := range proposalIDs {
		prefixes = append(prefixes, fmt.Sprintf("proposals/%s/", pid))
	}

	for _, prefix := range prefixes {
		paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
			Bucket: aws.String(s.bucket),
			Prefix: aws.String(prefix),
		})
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				break // best-effort: skip this prefix on error
			}
			for _, obj := range page.Contents {
				toDelete = append(toDelete, types.ObjectIdentifier{Key: obj.Key})
			}
		}
	}

	if len(toDelete) == 0 {
		return nil
	}

	// Delete in batches of 1000 (S3 API limit)
	const batchSize = 1000
	for i := 0; i < len(toDelete); i += batchSize {
		end := i + batchSize
		if end > len(toDelete) {
			end = len(toDelete)
		}
		_, _ = s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(s.bucket),
			Delete: &types.Delete{Objects: toDelete[i:end], Quiet: aws.Bool(true)},
		})
	}
	return nil
}

// buildS3Key constructs the S3 object key following the architecture path convention:
//
//	users/{user_id}/logo.{ext}
//	proposals/{proposal_id}/case/{block_id}.{ext}
//	proposals/{proposal_id}/team/{block_id}.{ext}
func buildS3Key(fileType, userID, proposalID, blockID, ext string) (string, error) {
	switch fileType {
	case "logo":
		if userID == "" {
			return "", ErrValidation
		}
		return fmt.Sprintf("users/%s/logo.%s", userID, ext), nil
	case "case_study":
		if proposalID == "" || blockID == "" {
			return "", ErrValidation
		}
		return fmt.Sprintf("proposals/%s/case/%s.%s", proposalID, blockID, ext), nil
	case "team_member":
		if proposalID == "" || blockID == "" {
			return "", ErrValidation
		}
		return fmt.Sprintf("proposals/%s/team/%s.%s", proposalID, blockID, ext), nil
	default:
		return "", ErrValidation
	}
}
