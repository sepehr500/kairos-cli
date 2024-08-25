package main

import (
	"context"
	"fmt"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	temporalEnums "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/workflow/v1"
	"go.temporal.io/api/workflowservice/v1"
	"log"
	"os"
	"strings"
	"time"
)

// ========================================
// Screen Constants
// ========================================
const (
	// App Header Height
	HEADER_HEIGHT = 1
	// App Search Input Height
	SEARCH_INPUT_HEIGHT = 1
)

// ========================================
// Search Logic and Rendering
// ========================================
var textInputWrapperStyle = lipgloss.NewStyle().Height(SEARCH_INPUT_HEIGHT)

type constructQueryStringParams struct {
	key   string
	value string
}

func (m model) constructQueryString(params constructQueryStringParams) string {
	return fmt.Sprintf("%s = '%s'", params.key, params.value)
}

func (m model) renderFooter() string {
	textInputWrapperStyle := textInputWrapperStyle.Width(m.viewport.Width)

	searchInputStyle := m.searchInput.View()
	return textInputWrapperStyle.Render(searchInputStyle)

}

// ========================================
// Table Logic and Rendering
// ========================================

var HeaderStyle = lipgloss.NewStyle().Padding(0, 0).Bold(true)
var EvenRowStyle = lipgloss.NewStyle().Padding(0, 0).Background(lipgloss.Color("#3b3b3b"))
var OddRowStyle = lipgloss.NewStyle().Padding(0, 0)

func (m model) renderHeader() string {
	headerStyle := lipgloss.NewStyle().Padding(0, 0).Width(m.viewport.Width).Height(HEADER_HEIGHT)
	header := "Workflow List"
	return headerStyle.Render(header)
}

func (m model) renderTable(workflows []*workflow.WorkflowExecutionInfo) string {

	tableSurroundStyle := lipgloss.NewStyle().Padding(0, 0).Height(m.viewport.Height - SEARCH_INPUT_HEIGHT - HEADER_HEIGHT)
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
	return tableSurroundStyle.Render(t.Render())
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
			PageSize: 30,
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

func (m model) updateVisibleWorkflowsBackgroundCmd() tea.Cmd {
	return tea.Tick(time.Second*3, func(_ time.Time) tea.Msg {
		temporalClient, _ := getTemporalClient()
		currentRunningExecutionIds := []string{}
		for _, execution := range m.workflows {
			if execution.GetCloseTime() == nil {
				currentRunningExecutionIds = append(currentRunningExecutionIds, "'"+execution.GetExecution().WorkflowId+"'")
			}
		}
		if len(currentRunningExecutionIds) == 0 {
			return updateVisibleWorkflowsMsg{workflows: []*workflow.WorkflowExecutionInfo{}}
		}
		query := fmt.Sprintf("WorkflowId IN (%s)", strings.Join(currentRunningExecutionIds, ","))
		queryResult, err := temporalClient.ListWorkflow(context.Background(), &workflowservice.ListWorkflowExecutionsRequest{
			Query:    query,
			PageSize: 20,
		})
		if err != nil {
			log.Fatalf("Failed to list workflows: %v", err)
		}
		return updateVisibleWorkflowsMsg{workflows: queryResult.GetExecutions()}

	})
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

type searchMode string

const (
	WORKFLOWTYPE   searchMode = "workflowType"
	WORKFLOWID     searchMode = "workflowId"
	WORKFLOWSTATUS searchMode = "workflowStatus"
)

type model struct {
	searchMode
	searchOptions              []string
	searchInput                textinput.Model
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
	textInput := textinput.New()
	textInput.Placeholder = "Search"
	// textInput.Prompt = ""
	return model{
		searchInput: textInput,
		ready:       false,
		workflows:   []*workflow.WorkflowExecutionInfo{},
		selected:    make(map[int]struct{}),
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
	view := m.renderHeader() + "\n" + m.renderTable(m.workflows) + "\n" + m.renderFooter()
	return view
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
		// Look for workflows that are in the current list and update them
		for _, updatedWorkflow := range msg.workflows {
			for i, currentWorkflow := range m.workflows {
				if updatedWorkflow.GetExecution().WorkflowId == currentWorkflow.GetExecution().WorkflowId {
					m.workflows[i] = updatedWorkflow
				}
			}
		}
		return m, m.updateVisibleWorkflowsBackgroundCmd()

	case updateWorkflowsMsg:
		m.workflows = msg.workflows
		return m, nil

	// Is it a key press?
	case tea.KeyMsg:
		if m.searchInput.Focused() && msg.String() == "enter" {
			m.searchStr = m.constructQueryString(constructQueryStringParams{key: "WorkflowType", value: m.searchInput.Value()})
			m.searchInput.Blur()
			m.searchInput.SetValue("")
			return m, m.refetchWorkflowsCmd()
		}
		if m.searchInput.Focused() && msg.String() != "esc" && msg.String() != "enter" && msg.String() != "ctrl+c" {
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			return m, cmd
		}
		// Cool, what was the actual key pressed?
		switch msg.String() {
		// Search by workflow type
		case "w":
			m.searchMode = WORKFLOWTYPE
			m.searchInput.Focus()
			return m, nil
		// If escape blur the search input
		case "esc":
			m.searchInput.Blur()
			return m, nil

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
		m.updateVisibleWorkflowsBackgroundCmd(),
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
