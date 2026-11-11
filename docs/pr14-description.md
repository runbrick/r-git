## Summary

The r-git interface now defaults to Simplified Chinese, with English selectable and saved from Settings without restarting. Existing settings migrate to Chinese; `R_GIT_LANG` provides an opt-in startup override for automation. Git output, commit text, refs, and paths remain unchanged.

## Java sources ported

No Java source files were ported. The git/vcs Chinese language pack is a separate non-open-source JetBrains component; the interface messages were translated directly and follow the glossary in `internal/i18n`.

## Design changes vs Java and why

The application uses typed English and Simplified Chinese message catalogues and passes the active catalogue from its window model to UI views and dialogs. This follows the Go/Viva ownership model and makes missing catalogue fields a compile-time error. Language changes update loaded date and tree-label caches. No dependencies were added.

## What was tested

- `gofmt -l .`: no output.
- `go vet ./...`: passed.
- `go build ./...`: passed.
- `go test ./...`: passed.
- `go test -tags e2e -count=2 ./e2e/...`: passed, including the Simplified Chinese commit flow.
- Bilingual toolbar/column layout tests passed at 1200, 1400, and 1600 logical pixels.
- Headless screenshots were captured at 1400 x 900 from the demo repo in both languages and themes; `layout` reported zero issues.

## Not done / follow-ups

None for PR14.
