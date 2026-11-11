PR12 visual evidence
====================

| Appearance | Before | After |
| --- | --- | --- |
| Dark | [before-dark.png](before-dark.png) | [after-dark.png](after-dark.png) |
| Light | [before-light.png](before-light.png) | [after-light.png](after-light.png) |

All four images show the same disposable repository through Viva's headless
build. The before images use a 1600 x 1000 logical window; the after images
use the default 1400 x 900 window (`ready 1400x900 scale=2`) to demonstrate
that the log header and metadata cells align at the default size. The fixture includes
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
   shot "<absolute-path>/after-dark.png"
   ```

The screenshot comparisons cover the shell, palette, density, default Commit
tool window, icon toolbar, changed-files tree, and commit details. Log graph
colors, ref-label styling, relative dates, author styling, and detailed commit
form design remain follow-up work for PR13.

The log column regression is also covered by an `app.Driver` test at 1200,
1400, and 1600 logical pixels, including resizing back down. It checks the
Author, Date, and Hash header/cell ranges, row bounds, and long-subject ellipsis.
