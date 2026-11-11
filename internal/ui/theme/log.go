// Copyright 2000-2024 JetBrains s.r.o. and contributors.
// Use of these VCS colors is governed by Apache 2.0; see NOTICE.
package theme

import (
	"math"

	"github.com/vivaui/viva/ui"
)

// LogColors are named VCS colors, kept with the application palette rather
// than embedded in painters. Graph saturation/brightness use the upstream
// defaults (both expUI themes inherit 0.4/0.65).
type LogColors struct {
	Unmatched, Local, Remote, Tag, Head ui.Color
}

func Log(t *ui.Theme) LogColors {
	if t.Background.Red < 0.5 {
		return LogColors{ui.RGB(111, 115, 122), ui.RGB(95, 173, 101), ui.RGB(181, 137, 236), ui.RGB(134, 138, 145), ui.RGB(245, 210, 115)}
	}
	return LogColors{ui.RGB(129, 133, 148), ui.RGB(54, 150, 80), ui.RGB(131, 77, 240), ui.RGB(108, 112, 126), ui.RGB(255, 175, 15)}
}

// GraphColor ports DefaultColorGenerator.calcColor, including Java Int
// overflow before rangeFix and HSBtoRGB's nearest-integer rounding. Zero is
// GraphColorManagerImpl.DEFAULT_COLOR, JBColor.BLACK (white in dark mode).
func GraphColor(id int, dark bool) ui.Color {
	if id == 0 {
		if dark {
			return ui.RGB(255, 255, 255)
		}
		return ui.RGB(0, 0, 0)
	}
	rangeFix := func(n int32) float64 {
		n %= 100
		if n < 0 {
			n = -n
		}
		return float64(n + 70)
	}
	r, g, b := rangeFix(int32(id)*200+30), rangeFix(int32(id)*130+50), rangeFix(int32(id)*90+100)
	hi, lo := max(r, g, b), min(r, g, b)
	h := 0.0
	if hi != lo {
		switch hi {
		case r:
			h = (g - b) / (hi - lo)
		case g:
			h = 2 + (b-r)/(hi-lo)
		default:
			h = 4 + (r-g)/(hi-lo)
		}
		h /= 6
		if h < 0 {
			h++
		}
	}
	h *= 6
	f := h - math.Floor(h)
	const v, s = 0.65, 0.4
	p, q, u := v*(1-s), v*(1-s*f), v*(1-s*(1-f))
	switch int(h) % 6 {
	case 0:
		r, g, b = v, u, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, u
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = u, p, v
	case 5:
		r, g, b = v, p, q
	}
	return ui.RGB(uint8(r*255+0.5), uint8(g*255+0.5), uint8(b*255+0.5))
}
