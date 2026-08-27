package azure

import (
	"testing"

	"github.com/spf13/afero"
)

func TestSwitchSubscription(t *testing.T) {
	fs := afero.NewMemMapFs()
	src := "\ufeff" + `{"installationId":"x","subscriptions":[
	  {"id":"1111","name":"Customer A","isDefault":true,"tenantId":"t1","extra":{"keep":"me"}},
	  {"id":"2222","name":"Customer B","isDefault":false,"tenantId":"t2"}]}`
	must(t, fs.MkdirAll("/az", 0o700))
	must(t, afero.WriteFile(fs, "/az/azureProfile.json", []byte(src), 0o600))
	must(t, afero.WriteFile(fs, "/az/msal_token_cache.json", []byte("{}"), 0o600))
	must(t, fs.MkdirAll("/az/logs", 0o700))
	must(t, fs.MkdirAll("/tmp", 0o700))
	if err := CopyConfig(fs, "/az", "/tmp"); err != nil {
		t.Fatal(err)
	}
	p, err := ReadProfile(fs, "/tmp")
	if err != nil {
		t.Fatal(err)
	}
	sub, err := p.Find("mer b")
	if err != nil || sub.ID != "2222" {
		t.Fatalf("find: %v %+v", err, sub)
	}
	if _, err := p.Find("customer"); err == nil {
		t.Fatal("expected ambiguous")
	}
	p.SetDefault(sub.ID)
	if err := p.Write(fs, "/tmp"); err != nil {
		t.Fatal(err)
	}

	p2, _ := ReadProfile(fs, "/tmp")
	if p2.Current() != "Customer B" {
		t.Fatalf("current = %q", p2.Current())
	}
	if p2.subs[0]["extra"] == nil {
		t.Fatal("unknown fields dropped")
	}
	if orig, _ := ReadProfile(fs, "/az"); orig.Current() != "Customer A" {
		t.Fatal("source profile modified")
	}
}

func TestDefaultGroup(t *testing.T) {
	fs := afero.NewMemMapFs()
	must(t, afero.WriteFile(fs, "/c/config", []byte("[core]\ngroup = wrong\n[defaults]\nlocation = westeurope\ngroup = my-rg\n"), 0o600))
	if g := DefaultGroup(fs, "/c"); g != "my-rg" {
		t.Fatalf("got %q", g)
	}
	if g := DefaultGroup(fs, "/none"); g != "" {
		t.Fatalf("got %q", g)
	}

	// Replace in place, other keys and sections untouched.
	must(t, SetDefaultGroup(fs, "/c", "other"))
	if b, _ := afero.ReadFile(fs, "/c/config"); string(b) != "[core]\ngroup = wrong\n[defaults]\ngroup = other\nlocation = westeurope\n" {
		t.Fatalf("replace:\n%s", b)
	}
	// Remove.
	must(t, SetDefaultGroup(fs, "/c", ""))
	if b, _ := afero.ReadFile(fs, "/c/config"); string(b) != "[core]\ngroup = wrong\n[defaults]\nlocation = westeurope\n" {
		t.Fatalf("remove:\n%s", b)
	}
	// Section missing: created after the existing content.
	must(t, afero.WriteFile(fs, "/d/config", []byte("[core]\noutput = json\n"), 0o600))
	must(t, SetDefaultGroup(fs, "/d", "new-rg"))
	if b, _ := afero.ReadFile(fs, "/d/config"); string(b) != "[core]\noutput = json\n[defaults]\ngroup = new-rg\n" {
		t.Fatalf("create section:\n%s", b)
	}
	// File missing.
	must(t, SetDefaultGroup(fs, "/e", "rg"))
	if g := DefaultGroup(fs, "/e"); g != "rg" {
		t.Fatalf("round-trip got %q", g)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
