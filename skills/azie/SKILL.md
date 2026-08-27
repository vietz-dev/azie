---
name: azie
description: "Use when running az (Azure CLI) commands: pick the right subscription without touching the global az context. Triggers: az, Azure subscription, az account set, wrong subscription, resource group default."
---

`az` reads its subscription from `AZURE_CONFIG_DIR`. `azie` gives every shell or command its own copy, so switching in one terminal never changes another. Never run `az account set` or `az login` to change context — it affects the whole machine or breaks the copy you are in.

1. **Check where you are.** `AZIE_ACTIVE=1` in the environment means this session is already pinned; `azie info` prints `subscription|group`. Run `az` as is.

2. **Not pinned?** Wrap each command: `azie ctx "<subscription>" -- az <args>`. The subscription is matched by name, id or unique substring; `az account list -o table` lists the candidates. Add `-g <group>` to also set the default resource group.

3. **Need a different subscription for a whole task?** Ask the user to start you inside it: `azie ctx "<subscription>" -- <agent command>`. Do not switch the session yourself.

4. **Auth errors** (`AADSTS`, "run az login"): ask the user to run `az login --tenant <tenant>` in a normal shell and restart the session; a login inside a pinned session is not shared.
