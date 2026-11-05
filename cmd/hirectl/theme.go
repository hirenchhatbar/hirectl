package main

import (
	"fmt"
	"image/color"
	"maps"
	"slices"

	"charm.land/lipgloss/v2"
)

// palette is a theme as hex colors ("#rrggbb"); "" means the terminal default.
type palette struct {
	Bg, Fg string // whole screen
	Muted  string // borders, disabled items, hints, function-key bar text; "" falls back to faint
	Accent string // headings, active function key
	Key    string // function-key labels
	SelBg  string // cursor row and function-key bar; "" falls back to reverse video
	SelFg  string
	Error  string
}

var themes = map[string]palette{
	"dracula": {
		Bg: "#282a36", Fg: "#f8f8f2", Muted: "#6272a4", Accent: "#bd93f9", Key: "#ff79c6",
		SelBg: "#44475a", SelFg: "#f8f8f2", Error: "#ff5555",
	},
	"terminal": {}, // terminal's own colors, attributes only
	"tokyonight-night": {
		Bg: "#1a1b26", Fg: "#c0caf5", Muted: "#565f89", Accent: "#7aa2f7", Key: "#bb9af7",
		SelBg: "#283457", SelFg: "#c0caf5", Error: "#f7768e",
	},
	"tokyonight-day": {
		Bg: "#e1e2e7", Fg: "#3760bf", Muted: "#848cb5", Accent: "#2e7de9", Key: "#9854f1",
		SelBg: "#b7c1e3", SelFg: "#3760bf", Error: "#f52a65",
	},
	"catppuccin-mocha": {
		Bg: "#1e1e2e", Fg: "#cdd6f4", Muted: "#6c7086", Accent: "#cba6f7", Key: "#f5c2e7",
		SelBg: "#313244", SelFg: "#cdd6f4", Error: "#f38ba8",
	},
	"catppuccin-latte": {
		Bg: "#eff1f5", Fg: "#4c4f69", Muted: "#9ca0b0", Accent: "#8839ef", Key: "#ea76cb",
		SelBg: "#ccd0da", SelFg: "#4c4f69", Error: "#d20f39",
	},
	"gruvbox-dark": {
		Bg: "#282828", Fg: "#ebdbb2", Muted: "#928374", Accent: "#83a598", Key: "#d3869b",
		SelBg: "#3c3836", SelFg: "#ebdbb2", Error: "#fb4934",
	},
	"gruvbox-light": {
		Bg: "#fbf1c7", Fg: "#3c3836", Muted: "#928374", Accent: "#076678", Key: "#8f3f71",
		SelBg: "#ebdbb2", SelFg: "#3c3836", Error: "#9d0006",
	},
	"nord": { // no official light variant
		Bg: "#2e3440", Fg: "#d8dee9", Muted: "#4c566a", Accent: "#88c0d0", Key: "#b48ead",
		SelBg: "#3b4252", SelFg: "#d8dee9", Error: "#bf616a",
	},
	"solarized-dark": {
		Bg: "#002b36", Fg: "#839496", Muted: "#586e75", Accent: "#268bd2", Key: "#d33682",
		SelBg: "#073642", SelFg: "#93a1a1", Error: "#dc322f",
	},
	"solarized-light": {
		Bg: "#fdf6e3", Fg: "#657b83", Muted: "#93a1a1", Accent: "#268bd2", Key: "#d33682",
		SelBg: "#eee8d5", SelFg: "#586e75", Error: "#dc322f",
	},
}

const defaultTheme = "dracula"

// themeName is the applied theme; the styles below are built from it by applyTheme.
var themeName string

var (
	paneStyle, selStyle, offStyle, headStyle, errStyle lipgloss.Style
	barStyle, fkeyStyle, activeStyle                   lipgloss.Style
)

func init() { _ = applyTheme(defaultTheme) }

// themeNames lists the themes, sorted.
func themeNames() []string { return slices.Sorted(maps.Keys(themes)) }

// applyTheme rebuilds the styles from the named theme. An unknown name applies
// dracula and returns an error, so a bad theme never blocks startup.
func applyTheme(name string) error {
	p, ok := themes[name]
	if !ok {
		applyTheme(defaultTheme)
		return fmt.Errorf("unknown theme %q, using %s", name, defaultTheme)
	}
	themeName = name
	c := lipgloss.Color // "" gives no color
	paneStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(c(p.Muted))
	headStyle = lipgloss.NewStyle().Bold(true).Foreground(c(p.Accent))
	errStyle = lipgloss.NewStyle().Foreground(c(p.Error))
	offStyle = lipgloss.NewStyle().Foreground(c(p.Muted)).Faint(p.Muted == "")
	selStyle = lipgloss.NewStyle().Background(c(p.SelBg)).Foreground(c(p.SelFg)).Reverse(p.SelBg == "")
	barStyle = lipgloss.NewStyle().Background(c(p.SelBg)).Foreground(c(p.Muted)).Reverse(p.SelBg == "")
	fkeyStyle = barStyle.Bold(true).Foreground(c(p.Key))
	activeStyle = barStyle.Bold(true).Foreground(c(p.Accent))
	return nil
}

// screenColors are the applied theme's terminal background and foreground; nil keeps the terminal's own.
func screenColors() (bg, fg color.Color) {
	p := themes[themeName]
	if p.Bg != "" {
		bg = lipgloss.Color(p.Bg)
	}
	if p.Fg != "" {
		fg = lipgloss.Color(p.Fg)
	}
	return bg, fg
}
