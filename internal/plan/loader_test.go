package plan

import (
	"context"
	"testing"
)

func TestLoader_JSON(t *testing.T) {
	l := NewLoader()
	p, err := l.Load(context.Background(), "../../testdata/plans/s3_create.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(p.ResourceChanges) != 1 {
		t.Fatalf("expected 1 resource change, got %d", len(p.ResourceChanges))
	}
	if p.ResourceChanges[0].Type != "aws_s3_bucket" {
		t.Errorf("expected type aws_s3_bucket, got %s", p.ResourceChanges[0].Type)
	}
}

func TestLoader_MissingFile(t *testing.T) {
	l := NewLoader()
	_, err := l.Load(context.Background(), "nonexistent.json")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoader_UnsupportedExtension(t *testing.T) {
	l := NewLoader()
	_, err := l.Load(context.Background(), "plan.unknown")
	if err == nil {
		t.Fatal("expected error for unsupported extension, got nil")
	}
}
