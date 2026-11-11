package model

import "strings"

// Ref name rules follow GitRefNameValidator, which implements
// git-check-ref-format for the one-level check the IDE applies to a branch
// name. The rules are:
//
//   - empty or all spaces is invalid
//   - cannot begin with '.', '-', or '/'
//   - cannot contain '~', '^', ':', '?', '*', '[', '\\', or the sequence "@{"
//   - cannot contain "//", "..", a '.' immediately before '/', or a '.'
//     immediately after '/'
//   - cannot end with '.', '/', or ".lock"
//   - cannot contain ASCII control characters (0..31, 127) or a space
//
// A name that contains a slash is checked as one string, so "user/branch"
// is valid and "user//branch" is not. The validator does not apply the
// at-rule ("a single @ is forbidden") from newer git; neither does the
// Java class, and a lone "@" passes there.

// ValidRefName reports whether name passes GitRefNameValidator.checkInput.
func ValidRefName(name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	return !refNameInvalid(name)
}

// refNameInvalid mirrors the combined VALIDATEPATTERNS search.
func refNameInvalid(name string) bool {
	// REPLACE: a space or a control character anywhere.
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c == ' ' || c < 0x20 || c == 0x7f {
			return true
		}
	}
	// DROP: begins with '.', '-', or '/'.
	switch name[0] {
	case '.', '-', '/':
		return true
	}
	// DROP: an invalid character or the "@{" sequence.
	for i := 0; i < len(name); i++ {
		switch name[i] {
		case '~', ':', '^', '?', '*', '"', '[', '\\':
			return true
		case '@':
			if i+1 < len(name) && name[i+1] == '{' {
				return true
			}
		}
	}
	// DROP: double slash, two dots, a dot next to a slash.
	if strings.Contains(name, "//") || strings.Contains(name, "..") {
		return true
	}
	if strings.Contains(name, "./") || strings.Contains(name, "/.") {
		return true
	}
	// ENDPATTERNS: ends with '.', '/', or ".lock".
	if strings.HasSuffix(name, ".") || strings.HasSuffix(name, "/") || strings.HasSuffix(name, ".lock") {
		return true
	}
	return false
}

// CleanBranchName applies GitRefNameValidator.cleanUpBranchName with the
// default replacement symbol "_". Spaces and control characters become
// "_"; the other illegal sequences are dropped; a trailing '.', '/', or
// ".lock" is removed. The cleanup can be applied repeatedly by the caller;
// one pass matches the Java method, which runs each replacement once.
func CleanBranchName(name string) string {
	return cleanBranchName(name, "_")
}

func cleanBranchName(name, spaceRepl string) string {
	// Drop illegal sequences first (DROP), then replace spaces and
	// controls (REPLACE), then strip the trailing pattern (ENDPATTERNS).
	var b strings.Builder
	b.Grow(len(name))
	for i := 0; i < len(name); {
		if i == 0 {
			switch name[0] {
			case '.', '-', '/':
				i++
				continue
			}
		}
		if name[i] == '@' && i+1 < len(name) && name[i+1] == '{' {
			i += 2
			continue
		}
		c := name[i]
		switch c {
		case '~', ':', '^', '?', '*', '"', '[', '\\':
			i++
			continue
		}
		// "//", "..", "./", "/." are each one alternative of the Java DROP
		// pattern. replaceAll consumes the whole match and does not
		// re-scan it, so the pair is dropped in one step.
		if i+1 < len(name) {
			two := name[i : i+2]
			if two == "//" || two == ".." || two == "./" || two == "/." {
				i += 2
				continue
			}
		}
		if c == ' ' || c < 0x20 || c == 0x7f {
			b.WriteString(spaceRepl)
			i++
			continue
		}
		// Keep the byte. Ref names are compared as bytes elsewhere; a
		// multibyte UTF-8 sequence is copied one byte at a time and stays
		// intact because none of its continuation bytes match the ASCII
		// rules above.
		b.WriteByte(c)
		i++
	}
	out := b.String()
	for {
		switch {
		case strings.HasSuffix(out, ".lock"):
			out = strings.TrimSuffix(out, ".lock")
		case strings.HasSuffix(out, "."), strings.HasSuffix(out, "/"):
			out = out[:len(out)-1]
		default:
			return out
		}
	}
}
