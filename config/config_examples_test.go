package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig_AllConfigExamples(t *testing.T) {
	root := filepath.Join("..", "config-examples")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read config-examples: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}
		path := filepath.Join(root, name)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			cfg, err := LoadConfig(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(cfg.Endpoints) == 0 {
				t.Fatal("expected endpoints")
			}
		})
	}
}
