package main

import (
	"github.com/charmbracelet/lipgloss"
)

var leftBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
var rightBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
var bottomBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())

// Each border is .5 characters wide, so we subtract 2 from the width and height
func (m model) focusedModeView() string {
	boxWidth := (m.viewport.Width / 2) - 2
	boxHeight := (m.viewport.Height * 2 / 3) - 2

	leftBox := leftBoxStyle.Width(boxWidth).Height(boxHeight).Render("Left box")

	// Right box
	rightBox := rightBoxStyle.Width(boxWidth).Height(boxHeight).Render("Right box")

	// Bottom box
	bottomBox := bottomBoxStyle.Width(m.viewport.Width - 3).Height(m.viewport.Height - boxHeight - 4).Render("Bottom box")
	return lipgloss.JoinVertical(lipgloss.Left, lipgloss.JoinHorizontal(lipgloss.Left, leftBox, rightBox), bottomBox)

}
