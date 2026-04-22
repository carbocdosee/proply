package service

import (
	"encoding/json"
	"testing"
)

// ─── ListTemplates ────────────────────────────────────────────────────────────

func TestListTemplates_ReturnsExactlyFive(t *testing.T) {
	tpls := ListTemplates()
	if len(tpls) != 5 {
		t.Errorf("expected 5 templates, got %d", len(tpls))
	}
}

func TestListTemplates_ContainsAllExpectedIDs(t *testing.T) {
	expected := []string{"web", "seo", "smm", "design", "consulting"}

	tpls := ListTemplates()
	got := make(map[string]bool, len(tpls))
	for _, tpl := range tpls {
		got[tpl.ID] = true
	}

	for _, id := range expected {
		if !got[id] {
			t.Errorf("expected template %q to be present in catalog", id)
		}
	}
}

func TestListTemplates_AllFieldsNonEmpty(t *testing.T) {
	for _, tpl := range ListTemplates() {
		if tpl.ID == "" {
			t.Error("found template with empty ID")
		}
		if tpl.Name == "" {
			t.Errorf("template %q has empty Name", tpl.ID)
		}
		if tpl.Description == "" {
			t.Errorf("template %q has empty Description", tpl.ID)
		}
		if len(tpl.BlockTypes) == 0 {
			t.Errorf("template %q has no BlockTypes", tpl.ID)
		}
	}
}

// ─── blocksForTemplate ────────────────────────────────────────────────────────

// templateSpec describes the expected structure for each known template.
var templateSpecs = []struct {
	id         string
	blockCount int
	blockTypes []string
}{
	{"web", 4, []string{"text", "price_table", "case_study", "terms"}},
	{"seo", 3, []string{"text", "price_table", "terms"}},
	{"smm", 3, []string{"text", "price_table", "terms"}},
	{"design", 4, []string{"text", "price_table", "case_study", "terms"}},
	{"consulting", 4, []string{"text", "price_table", "team_member", "terms"}},
}

func TestBlocksForTemplate_KnownTemplates(t *testing.T) {
	for _, tc := range templateSpecs {
		tc := tc // capture range variable
		t.Run(tc.id, func(t *testing.T) {
			blocks, err := blocksForTemplate(tc.id)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(blocks) != tc.blockCount {
				t.Errorf("block count: want %d, got %d", tc.blockCount, len(blocks))
			}

			for i, b := range blocks {
				// Correct block type in order
				if string(b.Type) != tc.blockTypes[i] {
					t.Errorf("block[%d]: type want %q, got %q", i, tc.blockTypes[i], b.Type)
				}
				// ID is a 32-char lowercase hex string
				if len(b.ID) != 32 {
					t.Errorf("block[%d]: ID must be 32 hex chars, got %d (%q)", i, len(b.ID), b.ID)
				}
				for _, c := range b.ID {
					if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
						t.Errorf("block[%d]: non-hex char %q in ID %q", i, c, b.ID)
					}
				}
				// Order matches position
				if b.Order != i {
					t.Errorf("block[%d]: order want %d, got %d", i, i, b.Order)
				}
				// Data must be valid JSON
				if !json.Valid(b.Data) {
					t.Errorf("block[%d]: Data is not valid JSON: %s", i, b.Data)
				}
			}
		})
	}
}

func TestBlocksForTemplate_UnknownID_ReturnsNil(t *testing.T) {
	blocks, err := blocksForTemplate("unknown-template-xyz")
	if err != nil {
		t.Errorf("expected nil error for unknown template id, got: %v", err)
	}
	if blocks != nil {
		t.Errorf("expected nil blocks for unknown template id, got %d blocks", len(blocks))
	}
}

func TestBlocksForTemplate_EmptyID_ReturnsNil(t *testing.T) {
	blocks, err := blocksForTemplate("")
	if err != nil {
		t.Errorf("expected nil error for empty template id, got: %v", err)
	}
	if blocks != nil {
		t.Errorf("expected nil blocks for empty template id, got %d blocks", len(blocks))
	}
}

func TestBlocksForTemplate_BlockIDsAreUnique(t *testing.T) {
	// Generate two sets of blocks for the same template.
	// Each call should produce different IDs (random UUIDs).
	blocksA, err := blocksForTemplate("web")
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	blocksB, err := blocksForTemplate("web")
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	for i := range blocksA {
		if blocksA[i].ID == blocksB[i].ID {
			t.Errorf("block[%d]: expected different IDs across calls, both got %q", i, blocksA[i].ID)
		}
	}
}

// ─── newBlockID ───────────────────────────────────────────────────────────────

func TestNewBlockID_Is32HexChars(t *testing.T) {
	id, err := newBlockID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(id) != 32 {
		t.Errorf("expected 32-char hex string, got %d chars: %q", len(id), id)
	}
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("non-lowercase-hex char %q in ID %q", c, id)
		}
	}
}

func TestNewBlockID_Uniqueness(t *testing.T) {
	const iterations = 200
	seen := make(map[string]struct{}, iterations)
	for i := 0; i < iterations; i++ {
		id, err := newBlockID()
		if err != nil {
			t.Fatalf("iteration %d: unexpected error: %v", i, err)
		}
		if _, dup := seen[id]; dup {
			t.Errorf("duplicate block ID generated after %d iterations: %q", i, id)
		}
		seen[id] = struct{}{}
	}
}
