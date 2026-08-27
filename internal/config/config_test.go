package config

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
)

func TestLoad(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	fs := afero.NewMemMapFs()

	s, err := Load(fs)
	if err != nil || s.Shell != "" || s.Prompt.Disable || s.Behavior.ValidateGroups != "partial" {
		t.Fatalf("defaults: %v %+v", err, s)
	}

	src := "shell: fish\nprompt:\n  disable: true\nhooks:\n  start_ctx: echo hi\n  stop_ctx: echo bye\nbehavior:\n  validate_groups: true\n"
	if err := afero.WriteFile(fs, filepath.Join("/xdg", "azie", "config.yaml"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err = Load(fs)
	if err != nil {
		t.Fatal(err)
	}
	if s.Shell != "fish" || !s.Prompt.Disable || s.Hooks.StartCtx != "echo hi" || s.Hooks.StopCtx != "echo bye" || s.Behavior.ValidateGroups != "true" {
		t.Fatalf("parsed: %+v", s)
	}
}
