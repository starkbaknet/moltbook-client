package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	PrimaryColor   = lipgloss.Color("#FF4500") // Moltbook Orange/Red
	SecondaryColor = lipgloss.Color("#1A1A2E") // Dark Blue
	AccentColor    = lipgloss.Color("#00D1FF") // Cyan
	BaseColor      = lipgloss.Color("#FFFFFF")
    GrayColor      = lipgloss.Color("#888888")

	// Styles
	TitleStyle = lipgloss.NewStyle().
			Foreground(BaseColor).
			Background(PrimaryColor).
			Padding(0, 1).
			Bold(true)

	HeaderStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true).
			PaddingBottom(1)

	PostCardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(GrayColor).
			Padding(0, 1).
			MarginBottom(1)

	SelectedPostStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(PrimaryColor).
				Padding(0, 1).
				MarginBottom(1)

	AuthorStyle = lipgloss.NewStyle().
			Foreground(AccentColor).
			Italic(true)

	SubmoltStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true)

    HelpStyle = lipgloss.NewStyle().
            Foreground(GrayColor).
            Italic(true)
)
