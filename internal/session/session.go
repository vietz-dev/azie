// Package session stores azie's per-shell history and the global last-used state.
package session

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

// Session is the history of one azie shell, kept in $AZURE_CONFIG_DIR/azie-session.json.
// Entries are appended on every switch, newest last.
type Session struct {
	Ctx []string `json:"ctx"`
	Rg  []string `json:"rg"`
}

// State is the cross-shell state in $XDG_STATE_HOME/azie/state.json.
type State struct {
	LastCtx string `json:"last_ctx"`
}

// StatePath is $XDG_STATE_HOME/azie/state.json, default ~/.local/state/azie/state.json.
func StatePath() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "azie", "state.json")
}

// Load reads the session of the shell using dir; a missing file yields an empty session.
func Load(fs afero.Fs, dir string) (Session, error) {
	var s Session
	return s, readJSON(fs, filepath.Join(dir, "azie-session.json"), &s)
}

func (s Session) Save(fs afero.Fs, dir string) error {
	return writeJSON(fs, filepath.Join(dir, "azie-session.json"), s)
}

func LoadState(fs afero.Fs) (State, error) {
	var s State
	return s, readJSON(fs, StatePath(), &s)
}

func SaveState(fs afero.Fs, s State) error {
	return writeJSON(fs, StatePath(), s)
}

// Previous returns the entry before the newest one, i.e. what `-` switches back to.
func Previous(history []string) (string, bool) {
	if len(history) < 2 {
		return "", false
	}
	return history[len(history)-2], true
}

func readJSON(fs afero.Fs, path string, v any) error {
	b, err := afero.ReadFile(fs, path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func writeJSON(fs afero.Fs, path string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err := fs.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return afero.WriteFile(fs, path, b, 0o600)
}
