PR13 visual evidence
====================

| Appearance | Before (main 12e55b1) | After |
| --- | --- | --- |
| Dark | [before-dark.png](before-dark.png) | [after-dark.png](after-dark.png) |
| Light | [before-light.png](before-light.png) | [after-light.png](after-light.png) |

Additional after captures select `Login returns bool`:
[dark](after-login-dark.png), [light](after-login-light.png).

All six captures use the default logical window (headless reports
`ready 1400x900 scale=2`) and the same coordinator-provided disposable demo
repository at `C:/Users/runbr/AppData/Local/Temp/rgit-demo/repo`. It has two
feature branches, a merge, tag `v0.1`, an origin remote, a modified `main.go`,
and an unversioned `notes.txt`. No repository content was changed for capture.
The reference is `rgit-specs/ref/rebased.png`, the Rebased IntelliJ New UI log.

Capture follows the PR12 procedure in [../README.md](../README.md): build
`go build -tags vivahead -o <temp>/r.exe ./cmd/r-git`, set `VIVA_HEADLESS=1`,
use a throwaway `HOME`/`USERPROFILE`, empty `GIT_CONFIG_GLOBAL` and
`GIT_CONFIG_SYSTEM`, and an isolated `R_GIT_CONFIG_DIR`. Its `settings.json`
is `{"version":1,"data":{"theme":"dark"}}` or the light variant.
The before binary was built from unmodified `origin/main` before any edits.
The original before images used:

```text
waitfor "Describe the repo"
waitfor "main.go"
waitfor "Changed files"
waitfor "login.go"
shot "<absolute-path>/<before-or-after>-<dark-or-light>.png"
quit
```

The refreshed after images use the same repository and correctly match the
metadata's topological order to the graph's order. The default first row is
`Start UI work`; `Login returns bool` is a parent of the merge, so its upward
edge correctly connects to that merge. The mismatch predated PR13, when
metadata was paged independently in Git's default date order. No changes to
the painter's correct head-edge rules were necessary. The after protocol is:

```text
waitfor "Start UI work"
waitfor "main.go"
waitfor "Changed files"
waitfor "ui.txt"
shot "<absolute-path>/after-<dark-or-light>.png"
click "Login returns bool"
waitfor "login.go"
shot "<absolute-path>/after-login-<dark-or-light>.png"
quit
```

Each shot waits for the selected commit's changed file to finish loading.
The toolbar spans the log and details panes, with fixed-width regex/case
toggles and a bordered User filter dropdown. Driver tests check the full
painted text width and clip bounds at 1200/1400/1600 logical pixels.

The after images show thin connected graph lanes, small filled nodes, dim
unselected merge rows, ref markers, absolute dates, the hidden hash column,
filter controls, icon actions, checkbox groups with counts, the compact
commit form, and commit metadata beneath changed files. The pinned Viva
textarea currently ignores `.Mono()`; that authorized follow-up is documented
in the PR rather than represented as completed visual work.
