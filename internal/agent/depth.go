package agent

// ResearchDepth controls the thoroughness vs speed trade-off for research.
type ResearchDepth string

const (
	DepthFast   ResearchDepth = "fast"
	DepthNormal ResearchDepth = "normal"
	DepthDeep   ResearchDepth = "deep"
)

// DepthConfig holds all tunable parameters for a given depth level.
type DepthConfig struct {
	SkipPreprocessor  bool
	TargetQueryCount  int
	MaxResultsPerQuery int
	PrimaryURLCount   int
	BackupURLCount    int
	TargetSources     int
	ContentTruncation int
	BiasTruncation    int
	EnableBias        bool
}

// ParseDepth converts a string to ResearchDepth, defaulting to DepthNormal.
func ParseDepth(s string) ResearchDepth {
	switch ResearchDepth(s) {
	case DepthFast, DepthNormal, DepthDeep:
		return ResearchDepth(s)
	default:
		return DepthNormal
	}
}

// GetDepthConfig returns the preset configuration for a given depth.
func GetDepthConfig(depth ResearchDepth) DepthConfig {
	switch depth {
	case DepthFast:
		return DepthConfig{
			SkipPreprocessor:   true,
			TargetQueryCount:   1,
			MaxResultsPerQuery: 5,
			PrimaryURLCount:    5,
			BackupURLCount:     0,
			TargetSources:      5,
			ContentTruncation:  15000,
			BiasTruncation:     10000,
			EnableBias:         false,
		}
	case DepthDeep:
		return DepthConfig{
			SkipPreprocessor:   false,
			TargetQueryCount:   5,
			MaxResultsPerQuery: 10,
			PrimaryURLCount:    20,
			BackupURLCount:     15,
			TargetSources:      18,
			ContentTruncation:  50000,
			BiasTruncation:     10000,
			EnableBias:         true,
		}
	default: // DepthNormal
		return DepthConfig{
			SkipPreprocessor:   false,
			TargetQueryCount:   3,
			MaxResultsPerQuery: 10,
			PrimaryURLCount:    15,
			BackupURLCount:     15,
			TargetSources:      12,
			ContentTruncation:  30000,
			BiasTruncation:     10000,
			EnableBias:         true,
		}
	}
}
