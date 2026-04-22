package slug

import (
	"strings"
	"sync"
	"testing"
)

// base62Charset is the expected alphabet for validation.
const base62Charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// isBase62 reports whether every character in s is in the Base62 alphabet.
func isBase62(s string) bool {
	for _, c := range s {
		if !strings.ContainsRune(base62Charset, c) {
			return false
		}
	}
	return true
}

// ─── Generate ────────────────────────────────────────────────────────────────

// TestGenerate_Is12Chars verifies that every generated slug is exactly 12 characters.
// Covers AC-4: slug always 12 characters.
func TestGenerate_Is12Chars(t *testing.T) {
	for i := 0; i < 50; i++ {
		s, err := Generate()
		if err != nil {
			t.Fatalf("Generate() error: %v", err)
		}
		if len(s) != slugLength {
			t.Errorf("iteration %d: expected length %d, got %d (slug=%q)", i, slugLength, len(s), s)
		}
	}
}

// TestGenerate_IsBase62Only verifies that generated slugs contain only Base62 characters.
// Covers AC-4: only Base62 symbols [0-9A-Za-z].
func TestGenerate_IsBase62Only(t *testing.T) {
	for i := 0; i < 50; i++ {
		s, err := Generate()
		if err != nil {
			t.Fatalf("Generate() error: %v", err)
		}
		if !isBase62(s) {
			t.Errorf("iteration %d: slug %q contains non-Base62 characters", i, s)
		}
	}
}

// TestGenerate_Uniqueness verifies that successive calls produce distinct slugs.
// With ~71 bits of entropy, collisions in 10 000 samples are astronomically unlikely.
func TestGenerate_Uniqueness(t *testing.T) {
	const n = 10_000
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		s, err := Generate()
		if err != nil {
			t.Fatalf("Generate() error at iteration %d: %v", i, err)
		}
		if _, dup := seen[s]; dup {
			t.Fatalf("collision after %d iterations: slug %q generated twice", i, s)
		}
		seen[s] = struct{}{}
	}
}

// TestGenerate_Concurrent_100_UniqueSlugss generates 100 slugs concurrently and
// asserts they are all unique. Mirrors the stress scenario described in AC-3
// (100 simultaneous publishes must produce unique slugs) at the slug-generation layer.
func TestGenerate_Concurrent_100_UniqueSlugs(t *testing.T) {
	const n = 100
	results := make([]string, n)
	errs := make([]error, n)

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			results[i], errs[i] = Generate()
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("goroutine %d: Generate() error: %v", i, err)
		}
	}

	seen := make(map[string]struct{}, n)
	for i, s := range results {
		if len(s) != slugLength {
			t.Errorf("goroutine %d: slug length want %d, got %d", i, slugLength, len(s))
		}
		if !isBase62(s) {
			t.Errorf("goroutine %d: slug %q has non-Base62 chars", i, s)
		}
		if _, dup := seen[s]; dup {
			t.Errorf("goroutine %d: duplicate slug %q", i, s)
		}
		seen[s] = struct{}{}
	}
}

// ─── GenerateWithRetry ───────────────────────────────────────────────────────

// TestGenerateWithRetry_SuccessFirstAttempt verifies that when no slug is taken
// the first candidate is returned immediately.
func TestGenerateWithRetry_SuccessFirstAttempt(t *testing.T) {
	calls := 0
	s, err := GenerateWithRetry(func(candidate string) (bool, error) {
		calls++
		return false, nil // nothing is taken
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s) != slugLength {
		t.Errorf("expected length %d, got %d", slugLength, len(s))
	}
	if calls != 1 {
		t.Errorf("isTaken called %d times, expected 1", calls)
	}
}

// TestGenerateWithRetry_SkipsTakenSlug verifies that a taken slug is skipped and
// the function retries with a fresh candidate.
func TestGenerateWithRetry_SkipsTakenSlug(t *testing.T) {
	calls := 0
	s, err := GenerateWithRetry(func(candidate string) (bool, error) {
		calls++
		// Report the first candidate as taken; accept the second.
		return calls == 1, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s == "" {
		t.Error("expected a non-empty slug on the second attempt")
	}
	if calls != 2 {
		t.Errorf("isTaken called %d times, expected 2", calls)
	}
}

// TestGenerateWithRetry_FailsAfterMaxRetries verifies that an error is returned
// when every candidate is reported as taken (exhausts maxRetries).
func TestGenerateWithRetry_FailsAfterMaxRetries(t *testing.T) {
	_, err := GenerateWithRetry(func(string) (bool, error) {
		return true, nil // all candidates are "taken"
	})
	if err == nil {
		t.Fatal("expected error when all slugs are taken, got nil")
	}
}
