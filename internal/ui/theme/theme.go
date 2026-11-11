// Package theme adapts the JetBrains expUI palette to Viva's semantic colors.
// Copyright 2000-2024 JetBrains s.r.o. and contributors.
// Use of the palette is governed by the Apache 2.0 license; see NOTICE.
package theme

import "github.com/vivaui/viva/ui"

// Dark is IntelliJ New UI's dark appearance. Palette names and UI assignments
// come from platform/platform-resources/src/themes/expUI/expUI_dark.theme.json.
func Dark() ui.Theme {
	t := ui.DarkTheme()
	t.Background = ui.RGB(30, 31, 34) // Gray1: editor and tool windows
	t.Card = ui.RGB(43, 45, 48)       // Gray2: window chrome
	t.CardElevated, t.Popover = t.Card, t.Card
	t.Foreground = ui.RGB(223, 225, 229)      // Gray12
	t.MutedForeground = ui.RGB(134, 138, 145) // Gray8
	t.Border = ui.RGB(57, 59, 64)             // Gray3
	t.Hover, t.Muted = t.Border, t.Border
	t.Input = ui.RGB(78, 81, 87)      // Gray5
	t.Primary = ui.RGB(53, 116, 240)  // Blue6
	t.Selection = ui.RGB(46, 67, 110) // Blue2
	t.Success = ui.RGB(95, 173, 101)  // Green7
	t.Danger = ui.RGB(219, 92, 92)    // Red7
	return finish(t)
}

// Light uses the matching expUI_light.theme.json palette and assignments.
func Light() ui.Theme {
	t := ui.LightTheme()
	t.Background = ui.RGB(255, 255, 255) // Gray14
	t.Card = ui.RGB(247, 248, 250)       // Gray13
	t.CardElevated, t.Popover = t.Background, t.Background
	t.Foreground = ui.RGB(39, 40, 46)         // Gray2
	t.MutedForeground = ui.RGB(108, 112, 126) // Gray6
	t.Border = ui.RGB(235, 236, 240)          // Gray12
	t.Hover, t.Muted = t.Border, t.Border
	t.Input = ui.RGB(168, 173, 189)     // Gray8
	t.Primary = ui.RGB(53, 116, 240)    // Blue4
	t.Selection = ui.RGB(212, 226, 255) // Blue11
	t.Success = ui.RGB(32, 138, 60)     // Green4
	t.Danger = ui.RGB(219, 59, 75)      // Red4
	return finish(t)
}

func finish(t ui.Theme) ui.Theme {
	t.CardForeground, t.PopoverForeground = t.Foreground, t.Foreground
	t.SelectionForeground = t.Foreground
	t.Accent, t.AccentForeground = t.Selection, t.Foreground
	t.Secondary, t.SecondaryForeground = t.Card, t.Foreground
	t.PrimaryForeground = ui.RGB(255, 255, 255)
	t.Ring = t.Primary
	t.Metrics.ControlHeight = 28
	t.Metrics.FontSizeS, t.Metrics.FontSizeM, t.Metrics.FontSizeL = 12, 13, 20
	t.Metrics.RadiusControl, t.Metrics.RadiusSurface = 4, 8
	t.Metrics.PadControlX, t.Metrics.PadSurface, t.Metrics.GapItem = 8, 12, 4
	return t
}
