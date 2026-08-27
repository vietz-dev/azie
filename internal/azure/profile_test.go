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
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
