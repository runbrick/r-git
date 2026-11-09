package command

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

type Version struct{ Major, Minor, Patch, Build int }

func ParseVersion(output string) (Version, error) {
	var v Version
	text := strings.TrimSpace(output)
	if !strings.HasPrefix(strings.ToLower(text), "git version ") {
		return v, fmt.Errorf("unsupported git version output %q", text)
	}
	parts := strings.Fields(text)[2:]
	if len(parts) == 0 {
		return v, fmt.Errorf("unsupported git version output %q", text)
	}
	numericText := strings.ReplaceAll(parts[0], ".windows.", ".")
	numeric := strings.Split(numericText, ".")
	values := make([]int, 0, 4)
	for _, item := range numeric {
		end := 0
		for end < len(item) && item[end] >= '0' && item[end] <= '9' {
			end++
		}
		if end == 0 {
			break
		}
		i, err := strconv.Atoi(item[:end])
		if err != nil {
			return v, fmt.Errorf("parse git version %q: %w", text, err)
		}
		values = append(values, i)
	}
	if len(values) < 2 {
		return v, fmt.Errorf("unsupported git version output %q", text)
	}
	for i := range values {
		if i > 3 {
			break
		}
		switch i {
		case 0:
			v.Major = values[i]
		case 1:
			v.Minor = values[i]
		case 2:
			v.Patch = values[i]
		case 3:
			v.Build = values[i]
		}
	}
	return v, nil
}

func MinimumVersion() Version {
	if runtime.GOOS == "windows" {
		return Version{Major: 2, Minor: 19, Patch: 2}
	}
	return Version{Major: 2, Minor: 17}
}

func (v Version) Supported() bool {
	m := MinimumVersion()
	if v.Major != m.Major {
		return v.Major > m.Major
	}
	if v.Minor != m.Minor {
		return v.Minor > m.Minor
	}
	if v.Patch != m.Patch {
		return v.Patch >= m.Patch
	}
	return v.Build >= m.Build
}

func (v Version) String() string {
	if v.Build != 0 {
		return fmt.Sprintf("%d.%d.%d.%d", v.Major, v.Minor, v.Patch, v.Build)
	}
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}
