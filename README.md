# r-git

A Go port of [Rebased](https://github.com/DetachHead/rebased), a git client based on the IntelliJ platform.
The UI uses the [Viva](https://github.com/vivaui/viva) desktop framework.
Viva is pinned to `v0.5.2-0.20261109155928-8b39edf779f6` because the page API used by this application is not available in a tagged release yet.

Work in progress. The port is split into small pull requests, one per module.

The interface defaults to Simplified Chinese (简体中文). Choose English in Settings > Language to change it without restarting; each choice is saved. For tests and automation, `R_GIT_LANG=en` or `R_GIT_LANG=zh-CN` selects the language for that process without changing the saved preference.

## Build

Build the application with:

```sh
go build ./cmd/r-git
```

The desktop UI uses Viva. Linux builds require XCB and Vulkan system libraries; CI installs their development packages on Ubuntu.

## Test

Run the local checks with:

```sh
sh scripts/check.sh
```

The checks cover formatting, vet, build, and tests. UI tests use Viva's headless app driver and do not open a window. Git tests create isolated temporary repositories and skip when Git is unavailable.

## Package layout

- `cmd/r-git`: application entry point and Viva UI.
- `internal/git/model`: core Git types without I/O.
- `internal/git/command`: Git CLI execution.
- `internal/git/repo`: repository state readers and parsers.
- `internal/git/ops`: branch, commit, stash, push, and rebase operations.
- `internal/graph`: commit graph layout.
- `internal/config`: settings and recent projects.
- `internal/ui`: Viva screens.
- `internal/testutil`: temporary Git repositories for tests.
