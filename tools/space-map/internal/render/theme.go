// Package render turns the gathered sky data into an animated SVG.
//
// Everything is CSS-only. GitHub strips <script> and camo blocks external
// references, so motion uses @keyframes and offset-path, never SMIL or JS.
package render

import "github.com/LarsLT/LarsLT/tools/space-map/internal/geo"

// Canvas layout.
const (
	Width  = 1000.0
	Height = 620.0

	HeaderH = 44.0
	MapW    = geo.MapW
	MapH    = geo.MapH
	MapY    = HeaderH
	FooterY = MapY + MapH

	LegendY      = FooterY + 22
	TickerY      = FooterY + 46
	TickerLineH  = 20
	Pad          = 16
	MaxTickerRow = 2
)

// Palette. The accent matches the blue already used across the profile README.
const (
	BgTop      = "#070b16"
	BgBottom   = "#0d1426"
	LandFill   = "#14243f"
	LandStroke = "#2c4c7d"
	Graticule  = "#1a2740"
	Night      = "#00040d"
	Accent     = "#58a6ff"
	Text       = "#e6edf3"
	Muted      = "#8b98a9"
	Dim        = "#5a6678"
	Aurora     = "#3ddc97"
	AuroraEdge = "#4fd1ff"
	Launch     = "#ff7a45"
	LaunchNext = "#ffc46b"
	Eclipse    = "#ffd166"
	ISS        = "#7fd1ff"
	Meteor     = "#cbb2ff"
	Star       = "#dce6f5"
	FontSans   = "'Segoe UI',Roboto,Helvetica,Arial,sans-serif"
	FontMono   = "ui-monospace,SFMono-Regular,Menlo,Consolas,monospace"
	TitleText  = "WHAT THE SKY IS DOING RIGHT NOW"
	StarCount  = 170
	StarSeed   = 1969 // Apollo 11
)
