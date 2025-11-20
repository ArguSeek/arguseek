package version

import (
	"fmt"
	"runtime/debug"
	"sync"
)

// injectedVersion is a simple package-level string variable
// that can be reliably set by -ldflags at build time.
// Example: go build -ldflags="-X 'arguseek/internal/version.injectedVersion=v1.2.3'"
var injectedVersion string

// Version provides the application's semantic version.
// It prioritizes injectedVersion (from ldflags), then ReadBuildInfo (from VCS),
// and finally a default "development" value.
//
// This hybrid approach follows Go 2024 best practices:
// - ldflags: explicit version control for release builds
// - ReadBuildInfo: automatic versioning for `go install` users
// - default: sensible fallback for development
var Version = sync.OnceValue(func() string {
	// 1. Check if version was explicitly injected via ldflags
	if injectedVersion != "" {
		return injectedVersion
	}

	// 2. Attempt to read build information from the binary
	if info, ok := debug.ReadBuildInfo(); ok {
		// Prefer the main module's version if available
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}

		// Fallback to VCS revision if Main.Version is not useful
		var vcsRevision, vcsTime, vcsModified string
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				vcsRevision = setting.Value
			case "vcs.time":
				vcsTime = setting.Value
			case "vcs.modified":
				vcsModified = setting.Value
			}
		}

		if vcsRevision != "" {
			// Use short hash (first 7 chars)
			shortHash := vcsRevision
			if len(vcsRevision) > 7 {
				shortHash = vcsRevision[:7]
			}

			if vcsModified == "true" {
				return fmt.Sprintf("%s-dirty", shortHash)
			}
			if vcsTime != "" {
				return fmt.Sprintf("%s (%s)", shortHash, vcsTime)
			}
			return shortHash
		}
	}

	// 3. Fallback to generic development version
	return "development"
})()

// GetFullBuildInfo provides comprehensive build details including
// Go version, module information, and VCS metadata.
// Useful for detailed --version output or debugging.
func GetFullBuildInfo() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.String()
	}
	return "Full build info not available (not built with module support or debug info stripped)"
}
