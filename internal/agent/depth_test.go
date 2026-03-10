package agent

import "testing"

func TestParseDepth_ValidValues(t *testing.T) {
	tests := []struct {
		input    string
		expected ResearchDepth
	}{
		{"fast", DepthFast},
		{"normal", DepthNormal},
		{"deep", DepthDeep},
	}

	for _, tt := range tests {
		got := ParseDepth(tt.input)
		if got != tt.expected {
			t.Errorf("ParseDepth(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestParseDepth_InvalidDefaults(t *testing.T) {
	tests := []string{"", "invalid", "FAST", "Normal", "turbo"}

	for _, input := range tests {
		got := ParseDepth(input)
		if got != DepthNormal {
			t.Errorf("ParseDepth(%q) = %q, want %q", input, got, DepthNormal)
		}
	}
}

func TestGetDepthConfig_NormalMatchesHardcoded(t *testing.T) {
	dc := GetDepthConfig(DepthNormal)

	// These values must match the previous hardcoded defaults
	if dc.TargetQueryCount != 3 {
		t.Errorf("TargetQueryCount = %d, want 3", dc.TargetQueryCount)
	}
	if dc.MaxResultsPerQuery != 10 {
		t.Errorf("MaxResultsPerQuery = %d, want 10", dc.MaxResultsPerQuery)
	}
	if dc.PrimaryURLCount != 15 {
		t.Errorf("PrimaryURLCount = %d, want 15", dc.PrimaryURLCount)
	}
	if dc.BackupURLCount != 15 {
		t.Errorf("BackupURLCount = %d, want 15", dc.BackupURLCount)
	}
	if dc.TargetSources != 12 {
		t.Errorf("TargetSources = %d, want 12", dc.TargetSources)
	}
	if dc.ContentTruncation != 30000 {
		t.Errorf("ContentTruncation = %d, want 30000", dc.ContentTruncation)
	}
	if dc.BiasTruncation != 10000 {
		t.Errorf("BiasTruncation = %d, want 10000", dc.BiasTruncation)
	}
	if !dc.EnableBias {
		t.Error("EnableBias = false, want true")
	}
	if dc.SkipPreprocessor {
		t.Error("SkipPreprocessor = true, want false")
	}
}

func TestGetDepthConfig_FastPreset(t *testing.T) {
	dc := GetDepthConfig(DepthFast)

	if !dc.SkipPreprocessor {
		t.Error("Fast: SkipPreprocessor = false, want true")
	}
	if dc.TargetQueryCount != 1 {
		t.Errorf("Fast: TargetQueryCount = %d, want 1", dc.TargetQueryCount)
	}
	if dc.MaxResultsPerQuery != 5 {
		t.Errorf("Fast: MaxResultsPerQuery = %d, want 5", dc.MaxResultsPerQuery)
	}
	if dc.EnableBias {
		t.Error("Fast: EnableBias = true, want false")
	}
}

func TestGetDepthConfig_DeepPreset(t *testing.T) {
	dc := GetDepthConfig(DepthDeep)

	if dc.SkipPreprocessor {
		t.Error("Deep: SkipPreprocessor = true, want false")
	}
	if dc.TargetQueryCount != 5 {
		t.Errorf("Deep: TargetQueryCount = %d, want 5", dc.TargetQueryCount)
	}
	if dc.PrimaryURLCount != 20 {
		t.Errorf("Deep: PrimaryURLCount = %d, want 20", dc.PrimaryURLCount)
	}
	if dc.TargetSources != 18 {
		t.Errorf("Deep: TargetSources = %d, want 18", dc.TargetSources)
	}
	if dc.ContentTruncation != 50000 {
		t.Errorf("Deep: ContentTruncation = %d, want 50000", dc.ContentTruncation)
	}
	if !dc.EnableBias {
		t.Error("Deep: EnableBias = false, want true")
	}
}
