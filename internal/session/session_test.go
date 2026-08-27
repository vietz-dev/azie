package session

import (
	"testing"

	"github.com/spf13/afero"
)

func TestSessionAndState(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/xdg")
	fs := afero.NewMemMapFs()

	s, err := Load(fs, "/dir")
	if err != nil || len(s.Ctx) != 0 {
		t.Fatalf("empty: %v %+v", err, s)
	}
	if _, ok := Previous(s.Ctx); ok {
		t.Fatal("expected no previous entry")
	}
	s.Ctx = append(s.Ctx, "A", "B")
	s.Rg = append(s.Rg, "", "rg1")
	if err := s.Save(fs, "/dir"); err != nil {
		t.Fatal(err)
	}
	s, err = Load(fs, "/dir")
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := Previous(s.Ctx); !ok || p != "A" {
		t.Fatalf("previous ctx = %q %v", p, ok)
	}
	if p, ok := Previous(s.Rg); !ok || p != "" {
		t.Fatalf("previous rg = %q %v", p, ok)
	}

	st, err := LoadState(fs)
	if err != nil || st.LastCtx != "" {
		t.Fatalf("empty state: %v %+v", err, st)
	}
	if err := SaveState(fs, State{LastCtx: "B"}); err != nil {
		t.Fatal(err)
	}
	if st, _ = LoadState(fs); st.LastCtx != "B" {
		t.Fatalf("state = %+v", st)
	}
	if ok, _ := afero.Exists(fs, "/xdg/azie/state.json"); !ok {
		t.Fatal("state file not at $XDG_STATE_HOME/azie/state.json")
	}
}
