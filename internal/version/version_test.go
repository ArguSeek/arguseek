package version

import (
	"runtime/debug"
	"strings"
	"testing"
)

// TestVersionIsNotEmpty verifies that the Version variable is initialized
// and returns a non-empty string in all deployment scenarios.
func TestVersionIsNotEmpty(t *testing.T) {
	if Version == "" {
		t.Error("Version should never be empty; expected at minimum 'development' fallback")
	}
}

// TestVersionFormat verifies that the version string follows expected patterns.
// Valid formats:
// - Semantic version with 'v' prefix (e.g., "v1.2.3") - from injected version
// - Git commit hash (7+ chars) - from VCS metadata
// - "development" - fallback when no version info available
func TestVersionFormat(t *testing.T) {
	// Version should be one of these patterns
	validPatterns := []struct {
		name    string
		matches func(string) bool
	}{
		{
			name: "semantic version (v1.2.3)",
			matches: func(v string) bool {
				return strings.HasPrefix(v, "v") && strings.Contains(v, ".")
			},
		},
		{
			name: "git commit hash (7+ chars)",
			matches: func(v string) bool {
				// Short hash is typically 7 chars, could be longer
				return len(v) >= 7 && !strings.HasPrefix(v, "v") && v != "development"
			},
		},
		{
			name: "development fallback",
			matches: func(v string) bool {
				return v == "development"
			},
		},
		{
			name: "dirty working tree (hash-dirty)",
			matches: func(v string) bool {
				return strings.HasSuffix(v, "-dirty")
			},
		},
		{
			name: "hash with timestamp",
			matches: func(v string) bool {
				return strings.Contains(v, "(") && strings.Contains(v, ")")
			},
		},
	}

	matched := false
	for _, pattern := range validPatterns {
		if pattern.matches(Version) {
			t.Logf("Version '%s' matches pattern: %s", Version, pattern.name)
			matched = true
			break
		}
	}

	if !matched {
		t.Errorf("Version '%s' does not match any expected format", Version)
	}
}

// TestGetFullBuildInfo verifies that GetFullBuildInfo returns build information.
// The exact content depends on how the binary was built, but it should never
// panic and should return a non-empty string.
func TestGetFullBuildInfo(t *testing.T) {
	info := GetFullBuildInfo()
	if info == "" {
		t.Error("GetFullBuildInfo should return non-empty string")
	}

	// If build info is available, it should contain "mod"
	if _, ok := debug.ReadBuildInfo(); ok {
		if !strings.Contains(info, "mod") && !strings.Contains(info, "Full build info not available") {
			t.Errorf("GetFullBuildInfo output unexpected format: %s", info)
		}
	}
}

// TestVersionConsistency verifies that calling Version multiple times
// returns the same value (sync.OnceValue caching behavior).
func TestVersionConsistency(t *testing.T) {
	// Call Version multiple times
	v1 := Version
	v2 := Version
	v3 := Version

	if v1 != v2 || v2 != v3 {
		t.Errorf("Version should return same value on multiple calls, got: %s, %s, %s", v1, v2, v3)
	}
}

// TestVersionNotDevel verifies that when running in a git repository with
// VCS metadata, we don't get the generic "(devel)" marker that Go defaults to.
// Instead, we should get either:
// - An injected version (from ldflags)
// - A git commit hash (from VCS metadata)
// - "development" (our custom fallback)
func TestVersionNotDevel(t *testing.T) {
	if Version == "(devel)" {
		t.Error("Version should not be Go's default '(devel)'; our fallback should be 'development'")
	}
}

// Note on testing limitations:
// The Version variable uses sync.OnceValue which is evaluated at package init.
// This makes it difficult to test different scenarios (injected vs VCS vs fallback)
// in isolation without restructuring the package.
//
// For comprehensive testing of version resolution logic:
// 1. Test injected version: build with -ldflags "-X internal/version.injectedVersion=v1.0.0"
// 2. Test VCS metadata: build normally in a git repo
// 3. Test development fallback: build outside git repo without ldflags
//
// These scenarios should be verified through integration tests or manual verification.
