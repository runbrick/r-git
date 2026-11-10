//go:build vivahead

package main

// Linked only into -tags vivahead builds: with VIVA_HEADLESS=1 the window
// serves the headless line protocol instead of opening a window.
import _ "github.com/vivaui/viva/headless/auto"
