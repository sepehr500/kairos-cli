package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	temporalEnums "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/workflow/v1"
	"go.temporal.io/api/workflowservice/v1"
)

// ========================================
// Table Logic and Rendering
// ========================================

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
		Headers("Status", "Type", "Id", "Start Time", "Close Time")
	for _, w := range workflows {
		workflowId := w.GetExecution().WorkflowId
		startTime := w.GetStartTime().AsTime().Format(time.RFC3339)
		closeTime := w.GetCloseTime().AsTime().Format(time.RFC3339)
		// If close time starts with 1970, it means the workflow is still running and has no close time
		if closeTime[:4] == "1970" {
			closeTime = "--"
		}
		t.Row(w.GetStatus().String(), w.GetType().Name, workflowId, startTime, closeTime)
	}
	return t.String()
}

// https://github.com/achannarasappa/ticker/blob/master/internal/ui/ui.go#L64

type backgroundUpdateWorkflowCountMsg struct {
	executionStatus temporalEnums.WorkflowExecutionStatus
	count           int64
}

func (m model) backgroundUpdateWorkflowCountCmd(exeuctionStatus temporalEnums.WorkflowExecutionStatus) tea.Cmd {
	return tea.Tick(time.Second*3, func(_ time.Time) tea.Msg {
		result := m.refetchWorkflowCountCmd(exeuctionStatus)()
		switch msg := result.(type) {
		case updateWorkflowCountMsg:
			return backgroundUpdateWorkflowCountMsg{executionStatus: exeuctionStatus, count: msg.count}

		}
		return nil
	})
}

type updateWorkflowsMsg struct {
	workflows []*workflow.WorkflowExecutionInfo
}

func (m model) refetchWorkflowsCmd() tea.Cmd {
	return func() tea.Msg {
		temporalClient, _ := getTemporalClient()
		query := m.searchStr
		queryResult, err := temporalClient.ListWorkflow(context.Background(), &workflowservice.ListWorkflowExecutionsRequest{
			Query:    query,
			PageSize: 20,
		})
		if err != nil {
			log.Fatalf("Failed to list workflows: %v", err)
		}
		result := queryResult.GetExecutions()
		return updateWorkflowsMsg{workflows: result}

	}
}

type updateVisibleWorkflowsMsg struct {
	workflows []*workflow.WorkflowExecutionInfo
}

func (m model) updateVisibleWorkflows() tea.Cmd {
	return func() tea.Msg {
		temporalClient, _ := getTemporalClient()
		query := m.searchStr
		queryResult, err := temporalClient.ListWorkflow(context.Background(), &workflowservice.ListWorkflowExecutionsRequest{
			Query:    query,
			PageSize: 20,
		})
		if err != nil {
			log.Fatalf("Failed to list workflows: %v", err)
		}
		// Update all workflows currently visible (in the model)
		for _, w := range m.workflows {
			for _, newW := range queryResult.GetExecutions() {
				if w.GetExecution().WorkflowId == newW.GetExecution().WorkflowId {
					w = newW
				}
			}
		}
		return updateVisibleWorkflowsMsg{workflows: m.workflows}
	}
}

type updateWorkflowCountMsg struct {
	executionStatus temporalEnums.WorkflowExecutionStatus
	count           int64
}

func (m model) refetchWorkflowCountCmd(executionStatus temporalEnums.WorkflowExecutionStatus) tea.Cmd {
	return func() tea.Msg {
		temporalClient, _ := getTemporalClient()
		statusQuery := fmt.Sprintf("ExecutionStatus = %d", executionStatus)
		query := m.searchStr
		if query == "" {
			query = statusQuery
		}
		if query != "" {
			query = fmt.Sprintf("%s AND %s", query, statusQuery)
		}
		queryResult, err := temporalClient.CountWorkflow(context.Background(), &workflowservice.CountWorkflowExecutionsRequest{
			Query: "",
		})
		if err != nil {
			log.Fatalf("Failed to count workflows: %v", err)
		}
		result := queryResult.GetCount()
		return updateWorkflowCountMsg{executionStatus: executionStatus, count: result}
	}
}

// ========================================
// Main Bubble Tea Control Loop
// ========================================

type model struct {
	ready                      bool
	searchStr                  string
	workflows                  []*workflow.WorkflowExecutionInfo // items on the to-do list
	cursor                     int                               // which to-do list item our cursor is pointing at
	selected                   map[int]struct{}                  // which to-do items are selected
	viewport                   viewport.Model
	staticVisibleWorkflowCount map[temporalEnums.WorkflowExecutionStatus]int64
	// This is the workflow count that is up to date in the background
	upToDateWorkflowCount map[temporalEnums.WorkflowExecutionStatus]int64
}

func initialModel() model {
	return model{
		ready:     false,
		workflows: []*workflow.WorkflowExecutionInfo{},
		selected:  make(map[int]struct{}),
		upToDateWorkflowCount: map[temporalEnums.WorkflowExecutionStatus]int64{
			temporalEnums.WORKFLOW_EXECUTION_STATUS_COMPLETED: 0,
			temporalEnums.WORKFLOW_EXECUTION_STATUS_FAILED:    0,
			temporalEnums.WORKFLOW_EXECUTION_STATUS_CANCELED:  0,
		},
		staticVisibleWorkflowCount: map[temporalEnums.WorkflowExecutionStatus]int64{
			temporalEnums.WORKFLOW_EXECUTION_STATUS_COMPLETED: 0,
			temporalEnums.WORKFLOW_EXECUTION_STATUS_FAILED:    0,
			temporalEnums.WORKFLOW_EXECUTION_STATUS_CANCELED:  0,
		},
	}
}

func (m model) View() string {
	m.viewport.SetContent(m.renderTable(m.workflows))
	return m.viewport.View()
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

	case updateWorkflowCountMsg:
		m.staticVisibleWorkflowCount[msg.executionStatus] = msg.count
		return m, nil
	case backgroundUpdateWorkflowCountMsg:
		m.upToDateWorkflowCount[msg.executionStatus] = msg.count
		return m, m.backgroundUpdateWorkflowCountCmd(msg.executionStatus)

	case updateVisibleWorkflowsMsg:
		m.workflows = msg.workflows
		return m, m.updateVisibleWorkflows()
	case updateWorkflowsMsg:
		m.workflows = msg.workflows
		return m, nil

	// Is it a key press?
	case tea.KeyMsg:

		// Cool, what was the actual key pressed?
		switch msg.String() {

		case "r":
			return m, m.refetchWorkflowsCmd()
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
	return tea.Batch(
		m.refetchWorkflowsCmd(),
		m.backgroundUpdateWorkflowCountCmd(temporalEnums.WORKFLOW_EXECUTION_STATUS_COMPLETED),
		m.backgroundUpdateWorkflowCountCmd(temporalEnums.WORKFLOW_EXECUTION_STATUS_FAILED),
		m.backgroundUpdateWorkflowCountCmd(temporalEnums.WORKFLOW_EXECUTION_STATUS_CANCELED),
		m.refetchWorkflowCountCmd(temporalEnums.WORKFLOW_EXECUTION_STATUS_COMPLETED),
		m.refetchWorkflowCountCmd(temporalEnums.WORKFLOW_EXECUTION_STATUS_FAILED),
		m.refetchWorkflowCountCmd(temporalEnums.WORKFLOW_EXECUTION_STATUS_CANCELED),
	)
}
func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
