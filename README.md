# azie

One shell, one Azure subscription: each shell gets its own Azure CLI context.

`az` reads its profile and token cache from `AZURE_CONFIG_DIR` (default `~/.azure`).
`azie` copies `~/.azure` to a temp dir, marks the chosen subscription as default there
and starts a shell with `AZURE_CONFIG_DIR` pointing at it. Other shells are unaffected.
The temp dir is removed when the shell exits.

```
go install github.com/vietz-dev/azie/cmd/azie@latest
mise use -g github:vietz-dev/azie   # or via mise, from the GitHub release

azie ctx                # pick subscription with fzf and spawn a shell
azie ctx "Customer A"   # same, by name, id or unique substring
azie ctx "Customer B"   # inside an azie shell: switch this shell in place
azie ctx -              # previous subscription (of this shell; outside a shell: the last one used)
azie ctx "Customer A" -g my-rg   # also set the default resource group
azie rg                 # inside an azie shell: pick default resource group with fzf ([defaults] group in the az config)
azie rg my-rg           # set it by name (validated, see settings)
azie rg -               # previous resource group of this shell
azie rg -u              # clear it
azie info               # print "subscription|group" (used by the prompt)
```

Supported shells: fish, zsh, bash (detected from the parent process or `$SHELL`, override with `AZIE_SHELL` or the settings file).
`AZIE_AZURE_HOME` overrides the source directory (`~/.azure`).

`az login` inside an azie shell only affects that shell. Log in once in a normal shell so
the token cache is inherited by all azie shells.

## Autocompletion

```
azie completion fish | source                                # fish, current session
azie completion fish > ~/.config/fish/completions/azie.fish  # fish, permanent
source <(azie completion zsh)                                # zsh (add to ~/.zshrc)
source <(azie completion bash)                               # bash (add to ~/.bashrc)
```

Subscription names are completed for `azie ctx`; resource groups for `azie rg` and
`azie ctx -g` (inside an azie shell only, via `az group list`).

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

## Settings

`azie edit-config` opens `$XDG_CONFIG_HOME/azie/config.yaml` (default `~/.config/azie/config.yaml`).
A missing file means defaults. All keys are optional:

```yaml
shell: fish            # override shell detection (fish|zsh|bash); env AZIE_SHELL still wins
prompt:
  disable: false       # env AZIE_PROMPT_DISABLE=1 still wins
hooks:
  start_ctx: ""        # shell snippet run when an azie shell starts (after rc files)
  stop_ctx: ""         # shell snippet run after the azie shell exits
behavior:
  validate_groups: partial   # true | false | partial (default partial)
```

`validate_groups` controls `azie rg <name>`: `false` sets the name as given without asking az;
`true` requires an exact match in `az group list`; `partial` also accepts a unique
case-insensitive substring match and opens fzf when several groups match.

State: each azie shell keeps its history in `$AZURE_CONFIG_DIR/azie-session.json` (for `ctx -`
and `rg -`); the last subscription used is stored in `$XDG_STATE_HOME/azie/state.json`
(default `~/.local/state/azie/state.json`).

Precedence: shell is `AZIE_SHELL` > `shell` > parent process > `$SHELL`; the prompt is
disabled by `AZIE_PROMPT_DISABLE=1` or `prompt.disable`. Hooks run with the shell's
environment (`AZURE_CONFIG_DIR`, `AZIE_ACTIVE`), `stop_ctx` via `<shell> -c`.
