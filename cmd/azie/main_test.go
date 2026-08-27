package main

import "testing"

func TestResolveGroup(t *testing.T) {
	calls := 0
	listGroups = func() ([]string, error) {
		calls++
		return []string{"prod-web", "prod-db", "staging"}, nil
	}
	cases := []struct{ mode, name, want string }{
		{"false", "anything", "anything"},
		{"true", "PROD-DB", "prod-db"},
		{"true", "prod", ""},
		{"partial", "staging", "staging"},
		{"partial", "stag", "staging"},
		{"partial", "web", "prod-web"},
		{"partial", "nope", ""},
	}
	for _, c := range cases {
		got, err := resolveGroup(c.mode, c.name)
		if (err == nil) != (c.want != "") || got != c.want {
			t.Errorf("%s %q: got %q, %v", c.mode, c.name, got, err)
		}
	}
	if calls != len(cases)-1 {
		t.Errorf("az called %d times, want %d (never for mode false)", calls, len(cases)-1)
	}
}

func TestCompleteGroups(t *testing.T) {
	listGroups = func() ([]string, error) { return []string{"prod-web", "staging"}, nil }
	if got, _ := completeGroups(false)(nil, nil, ""); got != nil {
		t.Errorf("outside an azie shell: got %v, want nothing", got)
	}
	if got, _ := completeGroups(true)(nil, nil, ""); len(got) != 2 || got[0] != "prod-web" {
		t.Errorf("inside an azie shell: got %v, want the az groups", got)
	}
}
