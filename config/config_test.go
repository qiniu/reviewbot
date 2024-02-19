package config

import (
	"fmt"
	"testing"
)

func TestConfig(t *testing.T) {
	repoConfig, err := NewConfig("config.yaml")
	if err != nil {
		t.Errorf("NewConfig() error = %v", err)
		return
	}

	for k, c := range repoConfig {
		fmt.Printf("%v: %+v \n", k, c)
	}

}
