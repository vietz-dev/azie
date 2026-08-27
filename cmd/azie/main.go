// azie spawns a shell bound to one Azure subscription, like kubie does for
// kubectl contexts. Each shell gets its own AZURE_CONFIG_DIR, so `az login`
// and `az account set` in one shell do not affect the others.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/vietz-dev/azie/internal/azure"
	"github.com/vietz-dev/azie/internal/shell"
)

// version is set by goreleaser via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	fs := afero.NewOsFs()
	active := os.Getenv("AZIE_ACTIVE") == "1"

	root := &cobra.Command{
		Use:           "azie",
		Short:         "kubie for the Azure CLI: one shell, one subscription",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(&cobra.Command{
		Use:   "ctx [subscription]",
		Short: "Spawn a shell for a subscription (fzf picker without argument); switches in place inside an azie shell",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := azure.Home()
			if active {
				dir = os.Getenv("AZURE_CONFIG_DIR")
			}
			p, err := azure.ReadProfile(fs, dir)
			if err != nil {
				return err
			}
			var sub azure.Subscription
			if len(args) == 1 {
				sub, err = p.Find(args[0])
			} else {
				var names []string
				for _, s := range p.Subscriptions() {
					names = append(names, s.Name)
				}
				var pick string
				if pick, err = fzf(names); err == nil {
					sub, err = p.Find(pick)
				}
			}
			if err != nil {
				return err
			}
			if active {
				p.SetDefault(sub.ID)
				return p.Write(fs, dir)
			}
			return shell.Spawn(fs, sub)
		},
	})

	var unset bool
	rg := &cobra.Command{
		Use:   "rg [resource-group]",
		Short: "Set the default resource group of this azie shell (fzf picker without argument)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !active {
				return fmt.Errorf("not inside an azie shell, run `azie ctx` first")
			}
			group := ""
			if unset {
				// fallthrough with empty group
			} else if len(args) == 1 {
				group = args[0]
			} else {
				var stderr bytes.Buffer
				c := exec.Command("az", "group", "list", "--query", "[].name", "-o", "tsv")
				c.Stderr = &stderr
				out, err := c.Output()
				if err != nil {
					if strings.Contains(stderr.String(), "AADSTS") || strings.Contains(stderr.String(), "az login") {
						return fmt.Errorf("azure session expired, run in this shell:\n  az login --tenant %q", currentTenant(fs))
					}
					return fmt.Errorf("az group list: %w\n%s", err, strings.TrimSpace(stderr.String()))
				}
				if group, err = fzf(strings.Fields(string(out))); err != nil {
					return err
				}
			}
			c := exec.Command("az", "configure", "--defaults", "group="+group)
			c.Stderr = os.Stderr
			return c.Run()
		},
	}
	rg.Flags().BoolVarP(&unset, "unset", "u", false, "clear the default resource group")
	root.AddCommand(rg)

	root.AddCommand(&cobra.Command{
		Use:   "info",
		Short: "Print subscription and resource group of the current azie shell (used by the prompt)",
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

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "azie:", err)
		os.Exit(1)
	}
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
