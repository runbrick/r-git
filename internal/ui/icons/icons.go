// Package icons contains only the JetBrains expUI icons used by r-git.
// Sources retain their Apache 2.0 headers; see LICENSE.txt and /NOTICE.
package icons

//go:generate go run github.com/vivaui/viva/cmd/viva icons -o generated.go -pkg icons -license HEADER.txt svg

import (
	"path"
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Pair chooses the original SVG for the page's appearance. Compiled icons keep
// their source colors, including file-type accents, in both appearances.
func Pair(c ui.C, light, dark gfx.Icon) gfx.Icon {
	if c.Theme().Background.Red < 0.5 {
		return dark
	}
	return light
}

// File selects the file-type icon without parsing SVGs during paint.
func File(c ui.C, name string) gfx.Icon {
	switch strings.ToLower(path.Ext(name)) {
	case ".md", ".markdown":
		return Pair(c, Markdown, Markdown_dark)
	case ".json":
		return Pair(c, Json, Json_dark)
	case ".java", ".kt", ".kts":
		return Pair(c, Java, Java_dark)
	case ".yaml", ".yml":
		return Pair(c, Yaml, Yaml_dark)
	case ".xml", ".xsd", ".xhtml":
		return Pair(c, Xml, Xml_dark)
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".icns":
		return Pair(c, Image, Image_dark)
	case ".ts", ".tsx":
		return Pair(c, Typescript, Typescript_dark)
	case ".js", ".jsx":
		return Pair(c, Javascript, Javascript_dark)
	default:
		return Pair(c, Text, Text_dark)
	}
}
