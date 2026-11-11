PR12 visual evidence
====================

| Appearance | Before | After |
| --- | --- | --- |
| Dark | [before-dark.png](before-dark.png) | [after-dark.png](after-dark.png) |
| Light | [before-light.png](before-light.png) | [after-light.png](after-light.png) |

All four images show the same disposable repository at a 1600 × 1000 logical
window size, rendered through Viva's headless build. The fixture includes
multiple branches, a tag, nested commit file changes, and two uncommitted files.
The reference for the layout is Rebased's `screenshot.png` (IntelliJ New UI).

The light baseline is the unmodified `main` application. For the dark baseline,
a temporary extraction of `main` explicitly supplied Viva's stock `DarkTheme`
to `PageConfig`: headless startup otherwise chooses light on this machine.
There are no layout changes in either baseline. The new application uses the
persisted `theme` setting for both appearances.

To capture the current application without opening a window:

1. Build `go build -tags vivahead -o <temp>/r.exe ./cmd/r-git`.
2. Use a disposable Git repository and empty temporary Git global/system config
   files. Set `VIVA_HEADLESS=1`, temporary `HOME` and `USERPROFILE`,
   `GIT_CONFIG_GLOBAL`, `GIT_CONFIG_SYSTEM`, and a separate `R_GIT_CONFIG_DIR`.
3. Write `{"version":1,"data":{"theme":"dark"}}` or the corresponding light
   setting to `<config>/settings.json` before launch.
4. Pipe these lines to `<temp>/r.exe <fixture>` (use forward slashes in paths):

   ```text
   waitfor "Add project overview"
   waitfor "README.md"
   waitfor "Changed files"
   resize 1600 1000
   shot "<absolute-path>/after-dark.png"
   ```

The screenshot comparisons cover the shell, palette, density, default Commit
tool window, icon toolbar, changed-files tree, and commit details. Log graph
colors, ref-label styling, relative dates, author columns, and detailed commit
form design remain follow-up work for PR13.
