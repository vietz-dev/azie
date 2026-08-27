// Package config loads the azie settings file, like kubie's ~/.kube/kubie.yaml.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

type Settings struct {
	Shell  string `yaml:"shell"` // fish|zsh|bash; AZIE_SHELL still wins
	Prompt struct {
		Disable bool `yaml:"disable"` // AZIE_PROMPT_DISABLE=1 still wins
	} `yaml:"prompt"`
	Hooks struct {
		StartCtx string `yaml:"start_ctx"` // shell snippet run when an azie shell starts (after rc files)
		StopCtx  string `yaml:"stop_ctx"`  // shell snippet run after the azie shell exits
	} `yaml:"hooks"`
	Behavior struct {
		ValidateGroups string `yaml:"validate_groups"` // true | false | partial
	} `yaml:"behavior"`
}

// Path is $XDG_CONFIG_HOME/azie/config.yaml, default ~/.config/azie/config.yaml.
func Path() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "azie", "config.yaml")
}

// Load reads the settings file. A missing file yields the defaults.
func Load(fs afero.Fs) (Settings, error) {
	var s Settings
	s.Behavior.ValidateGroups = "partial"
	b, err := afero.ReadFile(fs, Path())
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := yaml.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("parse %s: %w", Path(), err)
	}
	return s, nil
}
