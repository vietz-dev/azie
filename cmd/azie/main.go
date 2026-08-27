// azie spawns a shell bound to one Azure subscription, like kubie does for
// kubectl contexts. Each shell gets its own AZURE_CONFIG_DIR, so `az login`
// and `az account set` in one shell do not affect the others.
package main

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/vietz-dev/azie/internal/azure"
	"github.com/vietz-dev/azie/internal/config"
	"github.com/vietz-dev/azie/internal/session"
	"github.com/vietz-dev/azie/internal/shell"
)

// version is set by goreleaser via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	fs := afero.NewOsFs()
	active := os.Getenv("AZIE_ACTIVE") == "1"
	// A broken settings file must not break `info`, which runs on every prompt,
	// so the error is only reported by the commands that need the settings.
	cfg, cfgErr := config.Load(fs)

	root := &cobra.Command{
		Use:           "azie",
		Short:         "kubie for the Azure CLI: one shell, one subscription",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	var group string
	ctx := &cobra.Command{
		Use:   "ctx [subscription|-]",
		Short: "Spawn a shell for a subscription (fzf picker without argument, - for the previous one); switches in place inside an azie shell",
		Args:  cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return subscriptionNames(fs, active), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cfgErr != nil {
				return cfgErr
			}
			dir := azure.Home()
			if active {
				dir = os.Getenv("AZURE_CONFIG_DIR")
			}
			p, err := azure.ReadProfile(fs, dir)
			if err != nil {
				return err
			}
			sess, err := session.Load(fs, dir)
			if err != nil {
				return err
			}
			var query string
			switch {
			case len(args) == 0:
				var names []string
				for _, s := range p.Subscriptions() {
					names = append(names, s.Name)
				}
				if query, err = fzf(names); err != nil {
					return err
				}
			case args[0] != "-":
				query = args[0]
			case active:
				var ok bool
				if query, ok = session.Previous(sess.Ctx); !ok {
					return fmt.Errorf("no previous subscription in this shell")
				}
			default:
				st, err := session.LoadState(fs)
				if err != nil {
					return err
				}
				if query = st.LastCtx; query == "" {
					return fmt.Errorf("no previous subscription, run `azie ctx <subscription>` first")
				}
			}
			sub, err := p.Find(query)
			if err != nil {
				return err
			}
			if err := session.SaveState(fs, session.State{LastCtx: sub.Name}); err != nil {
				return err
			}
			if !active {
				return shell.Spawn(fs, cfg, sub, group)
			}
			p.SetDefault(sub.ID)
			if err := p.Write(fs, dir); err != nil {
				return err
			}
			sess.Ctx = append(sess.Ctx, sub.Name)
			if cmd.Flags().Changed("group") {
				if err := azure.SetDefaultGroup(fs, dir, group); err != nil {
					return err
				}
				sess.Rg = append(sess.Rg, group)
			}
			return sess.Save(fs, dir)
		},
	}
	ctx.Flags().StringVarP(&group, "group", "g", "", "also set the default resource group")
	_ = ctx.RegisterFlagCompletionFunc("group", completeGroups(active))
	root.AddCommand(ctx)

	var unset bool
	rg := &cobra.Command{
		Use:   "rg [resource-group|-]",
		Short: "Set the default resource group of this azie shell (fzf picker without argument, - for the previous one)",
		Args:  cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return completeGroups(active)(cmd, args, toComplete)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !active {
				return fmt.Errorf("not inside an azie shell, run `azie ctx` first")
			}
			if cfgErr != nil {
				return cfgErr
			}
			dir := os.Getenv("AZURE_CONFIG_DIR")
			sess, err := session.Load(fs, dir)
			if err != nil {
				return err
			}
			group := ""
			switch {
			case unset:
			case len(args) == 0:
				groups, err := listGroups()
				if err != nil {
					return err
				}
				if group, err = fzf(groups); err != nil {
					return err
				}
			case args[0] == "-":
				var ok bool
				if group, ok = session.Previous(sess.Rg); !ok {
					return fmt.Errorf("no previous resource group in this shell")
				}
			default:
				if group, err = resolveGroup(cfg.Behavior.ValidateGroups, args[0]); err != nil {
					return err
				}
			}
			if err := azure.SetDefaultGroup(fs, dir, group); err != nil {
				return err
			}
			sess.Rg = append(sess.Rg, group)
			return sess.Save(fs, dir)
		},
	}
	rg.Flags().BoolVarP(&unset, "unset", "u", false, "clear the default resource group")
	root.AddCommand(rg)

	root.AddCommand(&cobra.Command{
		Use:               "info",
		Short:             "Print subscription and resource group of the current azie shell (used by the prompt)",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !active {
				return fmt.Errorf("not inside an azie shell")
			}
			dir := os.Getenv("AZURE_CONFIG_DIR")
			p, err := azure.ReadProfile(fs, dir)
			if err != nil {
				return err
			}
			if g := azure.DefaultGroup(fs, dir); g != "" {
				fmt.Printf("%s|%s\n", p.Current(), g)
			} else {
				fmt.Println(p.Current())
			}
			return nil
		},
	})

	root.AddCommand(&cobra.Command{
		Use:               "edit-config",
		Short:             "Open the settings file (" + config.Path() + ") in $VISUAL, $EDITOR or vi",
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			path := config.Path()
			if err := fs.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				return err
			}
			if ok, _ := afero.Exists(fs, path); !ok {
				if err := afero.WriteFile(fs, path, nil, 0o600); err != nil {
					return err
				}
			}
			editor := strings.Fields(cmp.Or(os.Getenv("VISUAL"), os.Getenv("EDITOR"), "vi"))
			c := exec.Command(editor[0], append(editor[1:], path)...) //nolint:gosec // editor is the user's own choice
			c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
			return c.Run()
		},
	})

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "azie:", err)
		os.Exit(1)
	}
}

// listGroups returns the resource group names of the current subscription via az.
// It is a variable so tests can stub the az call.
var listGroups = func() ([]string, error) {
	var stderr bytes.Buffer
	c := exec.Command("az", "group", "list", "--query", "[].name", "-o", "tsv")
	c.Stderr = &stderr
	out, err := c.Output()
	if err != nil {
		if strings.Contains(stderr.String(), "AADSTS") || strings.Contains(stderr.String(), "az login") {
			return nil, fmt.Errorf("azure session expired, run in this shell:\n  az login --tenant %q", currentTenant(afero.NewOsFs()))
		}
		return nil, fmt.Errorf("az group list: %w\n%s", err, strings.TrimSpace(stderr.String()))
	}
	return strings.Fields(string(out)), nil
}

// subscriptionNames returns the subscription names for shell completion, plus
// "-" for the previous one. Errors are swallowed: completion must stay silent.
func subscriptionNames(fs afero.Fs, active bool) []string {
	dir := azure.Home()
	if active {
		dir = os.Getenv("AZURE_CONFIG_DIR")
	}
	p, err := azure.ReadProfile(fs, dir)
	if err != nil {
		return nil
	}
	names := []string{"-"}
	for _, s := range p.Subscriptions() {
		names = append(names, s.Name)
	}
	return names
}

// completeGroups completes resource group names via az, but only inside an azie
// shell: outside there is no subscription to list groups for.
func completeGroups(active bool) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		if !active {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		groups, _ := listGroups()
		return groups, cobra.ShellCompDirectiveNoFileComp
	}
}

// resolveGroup validates name according to behavior.validate_groups:
// "false" takes it as is, "true" needs an exact match, "partial" also accepts a
// unique case-insensitive substring match and lets fzf decide between several.
func resolveGroup(mode, name string) (string, error) {
	if mode == "false" {
		return name, nil
	}
	groups, err := listGroups()
	if err != nil {
		return "", err
	}
	var partial []string
	for _, g := range groups {
		if strings.EqualFold(g, name) {
			return g, nil
		}
		if mode == "partial" && strings.Contains(strings.ToLower(g), strings.ToLower(name)) {
			partial = append(partial, g)
		}
	}
	switch len(partial) {
	case 0:
		return "", fmt.Errorf("no resource group matches %q", name)
	case 1:
		return partial[0], nil
	}
	return fzf(partial)
}

func currentTenant(fs afero.Fs) string {
	p, err := azure.ReadProfile(fs, os.Getenv("AZURE_CONFIG_DIR"))
	if err != nil {
		return ""
	}
	for _, s := range p.Subscriptions() {
		if s.IsDefault {
			return s.TenantID
		}
	}
	return ""
}

// fzf lets the user pick one of items interactively.
func fzf(items []string) (string, error) {
	if len(items) == 0 {
		return "", fmt.Errorf("nothing to choose from")
	}
	c := exec.Command("fzf", "--height", "40%", "--reverse")
	c.Stdin = strings.NewReader(strings.Join(items, "\n"))
	c.Stderr = os.Stderr
	var out bytes.Buffer
	c.Stdout = &out
	var exit *exec.ExitError
	if err := c.Run(); err != nil {
		if errors.As(err, &exit) {
			return "", fmt.Errorf("nothing selected")
		}
		return "", fmt.Errorf("fzf: %w (install fzf or pass the name as argument)", err)
	}
	return strings.TrimSpace(out.String()), nil
}
