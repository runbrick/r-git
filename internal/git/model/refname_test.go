package model

import "testing"

// Cases ported from GitRefNameValidatorTest: the valid set, the invalid
// set, and one name per illegal character.
func TestValidRefName(t *testing.T) {
	valid := []string{
		"branch",
		"new_branch",
		"user/branch",
		"user/branch/sub_branch",
		"complex.branch.name",
		"refs/for/master%topic=my-cool-feature,r=some-reviewer",
		"b-ranch",
		"+branch",
		"teslá",
	}
	for _, name := range valid {
		if !ValidRefName(name) {
			t.Errorf("ValidRefName(%q) = false, want true", name)
		}
	}

	invalid := []string{
		".branch",
		"/branch",
		".",
		"branch/",
		"branch.",
		"branch.lock",
		"branch..name",
		"..name",
		"..branch",
		"",
		"  ",
		"-branch",
		"user//branch",
		"user/.hidden",
		"user/name./sub",
		"bra@{nch",
	}
	illegal := []string{" ", "~", "^", ":", "?", "*", "[", "@{", "\\", "\""}
	for _, ch := range illegal {
		invalid = append(invalid, "bra"+ch+"nch")
	}
	for c := 0; c <= 31; c++ {
		invalid = append(invalid, "bra"+string(rune(c))+"nch")
	}
	invalid = append(invalid, "bra\u007fnch")

	for _, name := range invalid {
		if ValidRefName(name) {
			t.Errorf("ValidRefName(%q) = true, want false", name)
		}
	}
}

func TestCleanBranchName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"feature", "feature"},
		{"my feature", "my_feature"},
		{".branch", "branch"},
		{"-branch", "branch"},
		{"branch.lock", "branch"},
		{"branch.", "branch"},
		{"branch/", "branch"},
		{"a..b", "ab"},
		{"a//b", "ab"},
	}
	for _, tt := range tests {
		if got := CleanBranchName(tt.in); got != tt.want {
			t.Errorf("CleanBranchName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
