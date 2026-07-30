// Package render turns the gathered sky data into an animated SVG. Everything
// is CSS-only: GitHub strips <script>, so motion is @keyframes and offset-path.
package render

import "github.com/LarsLT/LarsLT/tools/space-map/internal/geo"

// Canvas layout. No title bar: the picture says what it is, so the only chrome
// is the legend and ticker below the map.
const (
	Width  = 1000.0
	Height = MapH + FooterH

	MapW    = geo.MapW
	MapH    = geo.MapH
	FooterY = MapH
	FooterH = 96.0

	LegendY      = FooterY + 22
	TickerY      = FooterY + 48
	TickerLineH  = 18
	Pad          = 16
	MaxTickerRow = 3
)

// Palette. The accent matches the blue already used across the profile README.
const (
	BgBottom      = "#0d1426"
	Ocean         = "#0c1a30"
	OceanPolar    = "#091426"
	LandFill      = "#1b3054"
	LandStroke    = "#3a628f"
	Graticule     = "#1a2740"
	Night         = "#00040d"
	Accent        = "#58a6ff"
	Text          = "#e6edf3"
	Muted         = "#8b98a9"
	Dim           = "#5a6678"
	AuroraCore    = "#3ddc97"
	AuroraEdge    = "#4fd1ff"
	Launch        = "#ff7a45"
	LaunchNext    = "#ffc46b"
	EclipseColour = "#ffd166"
	ISS           = "#7fd1ff"
	Meteor        = "#cbb2ff"
	SunGlow       = "#ffd98a"
	SunCore       = "#fff3c4"
	FontSans      = "'Segoe UI',Roboto,Helvetica,Arial,sans-serif"
	FontMono      = "ui-monospace,SFMono-Regular,Menlo,Consolas,monospace"
	TitleText     = "What the sky is doing right now"
)
