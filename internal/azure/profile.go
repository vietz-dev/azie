// Package azure reads and writes the Azure CLI config directory (profile, config).
package azure

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
)

// Subscription is one entry of azureProfile.json.
type Subscription struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
	TenantID  string `json:"tenantId"`
}

type Profile struct {
	raw  map[string]json.RawMessage
	subs []map[string]any // raw entries so unknown fields survive a rewrite
}

// Home is the source config dir: AZIE_AZURE_HOME or ~/.azure.
func Home() string {
	if d := os.Getenv("AZIE_AZURE_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".azure")
}

func ReadProfile(fs afero.Fs, dir string) (*Profile, error) {
	b, err := afero.ReadFile(fs, filepath.Join(dir, "azureProfile.json"))
	if err != nil {
		return nil, fmt.Errorf("read azure profile (run `az login` first?): %w", err)
	}
	b = []byte(strings.TrimPrefix(string(b), "\ufeff")) // az writes a UTF-8 BOM
	p := &Profile{raw: map[string]json.RawMessage{}}
	if err := json.Unmarshal(b, &p.raw); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(p.raw["subscriptions"], &p.subs); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Profile) Write(fs afero.Fs, dir string) error {
	subs, err := json.Marshal(p.subs)
	if err != nil {
		return err
	}
	p.raw["subscriptions"] = subs
	b, err := json.Marshal(p.raw)
	if err != nil {
		return err
	}
	return afero.WriteFile(fs, filepath.Join(dir, "azureProfile.json"), append([]byte("\ufeff"), b...), 0o600)
}

func (p *Profile) Subscriptions() []Subscription {
	out := make([]Subscription, 0, len(p.subs))
	for _, s := range p.subs {
		id, _ := s["id"].(string)
		name, _ := s["name"].(string)
		def, _ := s["isDefault"].(bool)
		tenant, _ := s["tenantId"].(string)
		out = append(out, Subscription{ID: id, Name: name, IsDefault: def, TenantID: tenant})
	}
	return out
}

// Find matches by exact id/name first, then by unique case-insensitive substring.
func (p *Profile) Find(query string) (Subscription, error) {
	subs := p.Subscriptions()
	q := strings.ToLower(query)
	var partial []Subscription
	for _, s := range subs {
		if strings.EqualFold(s.ID, query) || strings.EqualFold(s.Name, query) {
			return s, nil
		}
		if strings.Contains(strings.ToLower(s.Name), q) {
			partial = append(partial, s)
		}
	}
	switch len(partial) {
	case 1:
		return partial[0], nil
	case 0:
		return Subscription{}, fmt.Errorf("no subscription matches %q", query)
	}
	names := make([]string, len(partial))
	for i, s := range partial {
		names[i] = s.Name
	}
	return Subscription{}, fmt.Errorf("%q is ambiguous: %s", query, strings.Join(names, ", "))
}

func (p *Profile) SetDefault(id string) {
	for _, s := range p.subs {
		s["isDefault"] = s["id"] == id
	}
}

func (p *Profile) Current() string {
	for _, s := range p.Subscriptions() {
		if s.IsDefault {
			return s.Name
		}
	}
	return ""
}

// CopyConfig copies the top-level files of src (profile, token cache, config)
// into dst. Subdirectories (logs, telemetry, ...) are skipped.
func CopyConfig(fs afero.Fs, src, dst string) error {
	entries, err := afero.ReadDir(fs, src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := afero.ReadFile(fs, filepath.Join(src, e.Name()))
		if err != nil {
			return err
		}
		if err := afero.WriteFile(fs, filepath.Join(dst, e.Name()), b, 0o600); err != nil {
			return err
		}
	}
	return nil
}

// DefaultGroup reads `[defaults] group = ...` from the az cli config file.
func DefaultGroup(fs afero.Fs, dir string) string {
	b, err := afero.ReadFile(fs, filepath.Join(dir, "config"))
	if err != nil {
		return ""
	}
	section := ""
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") {
			section = line
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok && section == "[defaults]" && strings.TrimSpace(k) == "group" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
