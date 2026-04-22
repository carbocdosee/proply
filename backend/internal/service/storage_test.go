package service

import (
	"context"
	"testing"
)

// ─── buildS3Key ───────────────────────────────────────────────────────────────

func TestBuildS3Key_Logo(t *testing.T) {
	key, err := buildS3Key("logo", "user-abc", "", "", "png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "users/user-abc/logo.png"
	if key != want {
		t.Errorf("key: want %q, got %q", want, key)
	}
}

func TestBuildS3Key_Logo_AllExtensions(t *testing.T) {
	for _, ext := range []string{"png", "jpg", "webp"} {
		key, err := buildS3Key("logo", "uid", "", "", ext)
		if err != nil {
			t.Errorf("ext %q: unexpected error: %v", ext, err)
		}
		if key == "" {
			t.Errorf("ext %q: expected non-empty key", ext)
		}
	}
}

func TestBuildS3Key_Logo_MissingUserID(t *testing.T) {
	_, err := buildS3Key("logo", "", "", "", "png")
	if err == nil {
		t.Error("expected error for missing userID, got nil")
	}
}

func TestBuildS3Key_CaseStudy(t *testing.T) {
	key, err := buildS3Key("case_study", "uid", "prop-1", "block-2", "jpg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "proposals/prop-1/case/block-2.jpg"
	if key != want {
		t.Errorf("key: want %q, got %q", want, key)
	}
}

func TestBuildS3Key_CaseStudy_MissingProposalID(t *testing.T) {
	_, err := buildS3Key("case_study", "uid", "", "block-2", "jpg")
	if err == nil {
		t.Error("expected error for missing proposalID, got nil")
	}
}

func TestBuildS3Key_CaseStudy_MissingBlockID(t *testing.T) {
	_, err := buildS3Key("case_study", "uid", "prop-1", "", "jpg")
	if err == nil {
		t.Error("expected error for missing blockID, got nil")
	}
}

func TestBuildS3Key_TeamMember(t *testing.T) {
	key, err := buildS3Key("team_member", "uid", "prop-1", "block-3", "webp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "proposals/prop-1/team/block-3.webp"
	if key != want {
		t.Errorf("key: want %q, got %q", want, key)
	}
}

func TestBuildS3Key_TeamMember_MissingIDs(t *testing.T) {
	_, err := buildS3Key("team_member", "uid", "", "", "png")
	if err == nil {
		t.Error("expected error for missing proposalID and blockID, got nil")
	}
}

func TestBuildS3Key_UnknownType(t *testing.T) {
	_, err := buildS3Key("avatar", "uid", "", "", "png")
	if err == nil {
		t.Error("expected error for unknown file type, got nil")
	}
}

// ─── allowedContentTypes ─────────────────────────────────────────────────────

func TestAllowedContentTypes_Accepted(t *testing.T) {
	for _, ct := range []string{"image/png", "image/jpeg", "image/webp"} {
		if _, ok := allowedContentTypes[ct]; !ok {
			t.Errorf("expected %q to be in allowedContentTypes", ct)
		}
	}
}

func TestAllowedContentTypes_Rejected(t *testing.T) {
	rejected := []string{
		"application/octet-stream",
		"application/x-msdownload",
		"text/plain",
		"image/gif",
		"image/bmp",
		"image/svg+xml",
	}
	for _, ct := range rejected {
		if _, ok := allowedContentTypes[ct]; ok {
			t.Errorf("expected %q to NOT be in allowedContentTypes", ct)
		}
	}
}

// ─── size constants ───────────────────────────────────────────────────────────

func TestSizeLimits_Logo2MB(t *testing.T) {
	const want = 2 * 1024 * 1024
	if maxLogoBytes != want {
		t.Errorf("maxLogoBytes: want %d, got %d", want, maxLogoBytes)
	}
}

func TestSizeLimits_CaseStudy5MB(t *testing.T) {
	const want = 5 * 1024 * 1024
	if maxCaseStudyBytes != want {
		t.Errorf("maxCaseStudyBytes: want %d, got %d", want, maxCaseStudyBytes)
	}
}

// ─── PresignUpload — StorageService not configured ────────────────────────────

func TestPresignUpload_NotConfigured(t *testing.T) {
	// StorageService created without S3 credentials → presignClient == nil
	svc := &StorageService{}
	_, err := svc.PresignUpload(context.Background(), PresignUploadInput{
		UserID:      "uid",
		FileType:    "logo",
		ContentType: "image/png",
		SizeBytes:   1024,
	})
	if err != ErrStorageNotConfigured {
		t.Errorf("want ErrStorageNotConfigured, got %v", err)
	}
}
