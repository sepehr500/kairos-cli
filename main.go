package main

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"go.temporal.io/api/workflow/v1"
)

type model struct {
	ready     bool
	search    string
	workflows []*workflow.WorkflowExecutionInfo // items on the to-do list
	cursor    int                               // which to-do list item our cursor is pointing at
	selected  map[int]struct{}                  // which to-do items are selected
	viewport  viewport.Model
}

var HeaderStyle = lipgloss.NewStyle().Padding(0, 0).Bold(true)
var EvenRowStyle = lipgloss.NewStyle().Padding(0, 0).Background(lipgloss.Color("#3b3b3b"))
var OddRowStyle = lipgloss.NewStyle().Padding(0, 0)

func (m model) renderTable(workflows []*workflow.WorkflowExecutionInfo) string {
	t := table.New().
		Border(lipgloss.HiddenBorder()).
		Width(m.viewport.Width).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == 0:
				return HeaderStyle
			case row%2 == 0:
				return EvenRowStyle
			default:
				return OddRowStyle
			}
		}).
		Headers("Status", "Type", "Id", "Start Time")
	for _, w := range workflows {
		workflowId := w.GetExecution().WorkflowId
		startTime := w.GetStartTime().AsTime().Format(time.RFC3339)
		t.Row(w.GetStatus().String(), w.GetType().Name, workflowId, startTime)
	}
	return t.String()
}

func initialModel() model {
	return model{
		ready:     false,
		workflows: []*workflow.WorkflowExecutionInfo{},
		search:    "",
		selected:  make(map[int]struct{}),
	}
}

func (m model) View() string {

	// Iterate over our choices
	// for i, choice := range m.workflows {
	// 	// Is the cursor pointing at this choice?
	// 	cursor := " " // no cursor
	// 	if m.cursor == i {
	// 		cursor = ">" // cursor!
	// 	}
	//
	// 	// Is this choice selected?
	// 	checked := " " // not selected
	// 	if _, ok := m.selected[i]; ok {
	// 		checked = "x" // selected!
	// 	}
	//
	// 	renderTable(m.workflows)
	// 	s += fmt.Sprintf("%s [%s] %s\n", cursor, checked, choice.Execution.GetWorkflowId())
	// }
	//
	// // The footer
	// s += "\nPress q to quit.\n"

	// Send the UI for rendering
	m.viewport.SetContent(m.renderTable(m.workflows))
	return m.viewport.View()
}

type listWorkflowsMsg struct {
	workflows []*workflow.WorkflowExecutionInfo
}

// https://github.com/achannarasappa/ticker/blob/master/internal/ui/ui.go#L64

func updateWorkflows(search string) tea.Cmd {
	return tea.Tick(time.Second*3, func(_ time.Time) tea.Msg {
		return listWorkflowsCmd(search)()
	})
}

func listWorkflowsCmd(search string) tea.Cmd {
	return func() tea.Msg {
		result := listWorkflows(search)
		return listWorkflowsMsg{workflows: result}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height)
			m.viewport.YPosition = 0
			m.ready = true

		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height

		}

	case listWorkflowsMsg:
		// log.Printf("Got workflows: %v", msg.workflows)
		m.workflows = msg.workflows
		return m, updateWorkflows(m.search)

	// Is it a key press?
	case tea.KeyMsg:

		// Cool, what was the actual key pressed?
		switch msg.String() {

		// These keys should exit the program.
		case "ctrl+c", "q":
			return m, tea.Quit

		// The "up" and "k" keys move the cursor up
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		// The "down" and "j" keys move the cursor down
		case "down", "j":
			if m.cursor < len(m.workflows)-1 {
				m.cursor++
			}

		// The "enter" key and the spacebar (a literal space) toggle
		// the selected state for the item that the cursor is pointing at.
		case "enter", " ":
			_, ok := m.selected[m.cursor]
			if ok {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = struct{}{}
			}
		}
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return m, nil
}

func (m model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return listWorkflowsCmd("")
}
func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
