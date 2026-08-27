# azie

`kubie` for the Azure CLI: each shell gets its own Azure subscription.

`az` reads its profile and token cache from `AZURE_CONFIG_DIR` (default `~/.azure`).
`azie` copies `~/.azure` to a temp dir, marks the chosen subscription as default there
and starts a shell with `AZURE_CONFIG_DIR` pointing at it. Other shells are unaffected.
The temp dir is removed when the shell exits.

```
go install github.com/vietz-dev/azie/cmd/azie@latest

azie ctx                # pick subscription with fzf and spawn a shell
azie ctx "Customer A"   # same, by name, id or unique substring
azie ctx "Customer B"   # inside an azie shell: switch this shell in place
azie rg                 # inside an azie shell: pick default resource group with fzf (az configure --defaults group=...)
azie rg -u              # clear it
azie info               # print "subscription|group" (used by the prompt)
```

Supported shells: fish, zsh, bash (detected via `$SHELL`, override with `AZIE_SHELL`).
`AZIE_AZURE_HOME` overrides the source directory (`~/.azure`).

`az login` inside an azie shell only affects that shell. Log in once in a normal shell so
the token cache is inherited by all azie shells.

## Prompt

azie prints `[subscription|group]` on its own line above your prompt and exports
`AZIE_CTX` with the same value. To integrate it into starship instead, set
`AZIE_PROMPT_DISABLE=1` and add the `env_var` module:

```toml
format = "$env_var$directory$git_branch..."

[env_var.AZIE_CTX]
format = "[$env_value]($style) "
style = "bold fg:#ca9ee6"
```
