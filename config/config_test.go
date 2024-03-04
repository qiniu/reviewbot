package config

import (
	"log"
	"testing"
)

func TestConfig(t *testing.T) {
	repoConfig, err := NewConfig("config.yaml")
	if err != nil {
		t.Errorf("NewConfig() error = %v", err)
		return
	}

	log.Printf("repoConfig: %+v\n", repoConfig)
	log.Printf("repoConfig: %+v\n", repoConfig.CustomConfig)
}
