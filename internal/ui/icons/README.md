JetBrains expUI icons
====================

These are the original Apache 2.0 SVGs from Rebased's
`platform/icons/src/expui/`. Copyright headers are preserved. Both the plain
SVG and its `_dark.svg` companion are included for each entry below. Names
are shortened locally to avoid collisions between upstream directories.

| Local name | Upstream path under `expui/` |
| --- | --- |
| add | general/add |
| branches | toolwindows/vcs |
| close | general/close |
| commit | toolwindows/commit |
| diff | vcs/diff |
| down | general/chevronDown |
| fetch | vcs/fetch |
| folder | nodes/folder |
| java | fileTypes/java |
| javascript | fileTypes/javaScript |
| json | fileTypes/json |
| left | vcs/arrowLeft |
| markdown | fileTypes/markdown |
| merge | vcs/merge |
| more | general/moreHorizontal |
| pull | vcs/update |
| push | vcs/push |
| rebase | vcs/changes |
| refresh | general/refresh |
| remotes | toolwindows/repositories |
| remove | vcs/remove |
| right | vcs/arrowRight |
| rollback | vcs/revert |
| settings | general/settings |
| stash | vcs/shelve |
| text | fileTypes/text |
| up | general/chevronUp |

Run `go generate ./internal/ui/icons` from the repository root to regenerate
`generated.go`. This uses the exact Viva version in `go.mod`; it does not need
an installed global Viva CLI. `HEADER.txt` supplies the generated license header,
`LICENSE.txt` contains Apache 2.0, and the repository `NOTICE` records attribution.

SVG parsing and compilation happen during generation. The application selects
compiled values for the current theme and never reads SVG files during paint.
