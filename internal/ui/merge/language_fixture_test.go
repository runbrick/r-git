package merge

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) { os.Setenv("R_GIT_LANG", "en"); os.Exit(m.Run()) }
