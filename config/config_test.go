package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfig(t *testing.T) {
	var testCases = []struct {
		name        string
		expectError bool
		rawConfig   string
		expected    Config
	}{
		{
			name:        "empty config",
			expectError: false,
			rawConfig:   ``,
			expected: Config{
				GlobalDefaultConfig: GlobalConfig{
					GithubReportType: GithubPRReview,
				},
			},
		},
		// TODO: add more test cases
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configDir := t.TempDir()
			configPath := filepath.Join(configDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tc.rawConfig), 0666); err != nil {
				t.Fatalf("fail to write prow config: %v", err)
			}

			c, err := NewConfig(configPath)
			if tc.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}

				if c.GlobalDefaultConfig.GithubReportType != tc.expected.GlobalDefaultConfig.GithubReportType {
					t.Errorf("expected %v, got %v", tc.expected.GlobalDefaultConfig.GithubReportType, c.GlobalDefaultConfig.GithubReportType)
				}

				if len(c.CustomConfig) != len(tc.expected.CustomConfig) {
					t.Errorf("expected %v, got %v", len(tc.expected.CustomConfig), len(c.CustomConfig))
				}
			}
		})
	}
}
