package main

import (
	"github.com/charmbracelet/lipgloss"
)

var leftBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
var rightBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
var bottomBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())

// Each border is .5 characters wide, so we subtract 2 from the width and height
func (m model) focusedModeView() string {
	var selectedWorkflow *workflowTableListItem = nil
	for _, workflow := range m.workflows {
		if workflow.workflow.Execution.WorkflowId == m.focusViewWorkflowId {
			selectedWorkflow = workflow
			break
		}
	}

	boxWidth := m.viewport.Width / 2
	boxHeight := m.viewport.Height * 2 / 3
	leftBoxStyle := leftBoxStyle.Width(boxWidth).Height(boxHeight).Padding(0, 0).Margin(0, 0)
	x, y := leftBoxStyle.GetFrameSize()
	rightBoxStyle := rightBoxStyle.Width(boxWidth-x*2).Height(boxHeight).Padding(0, 0).Margin(0, 0)

	// Bottom box
	bottomBoxStyle := bottomBoxStyle.Width(m.viewport.Width - x).Height(m.viewport.Height - boxHeight - y*2)
	return lipgloss.JoinVertical(lipgloss.Left, lipgloss.JoinHorizontal(lipgloss.Left, leftBoxStyle.Render(selectedWorkflow.workflow.String()), rightBoxStyle.Render("right box")), bottomBoxStyle.Render("bottom box"))

}
