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
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"log"
	"os"
	"strings"
	"time"
)

// ========================================
// Status Styles
// ========================================

// Status String to style map

type ExecutionStatusStyleInfo struct {
	displayName string
	icon        string
	color       string
}

var statusToStyleMap = map[string]ExecutionStatusStyleInfo{
	temporalEnums.WORKFLOW_EXECUTION_STATUS_COMPLETED.String(): {
		displayName: "Completed",
		icon:        "✅",
		color:       "#00ff00",
	},
	temporalEnums.WORKFLOW_EXECUTION_STATUS_FAILED.String(): {
		displayName: "Failed",
		icon:        "❌",
		color:       "#ff0000",
	},
	temporalEnums.WORKFLOW_EXECUTION_STATUS_CANCELED.String(): {
		displayName: "Canceled",
		icon:        "✋",
		color:       "#808080",
	},
	temporalEnums.WORKFLOW_EXECUTION_STATUS_RUNNING.String(): {
		displayName: "Running",
		icon:        "🏃",
		color:       "#00ff00",
	},
	temporalEnums.WORKFLOW_EXECUTION_STATUS_TERMINATED.String(): {
		displayName: "Terminated",
		// Skull icon
		icon:  "💀",
		color: "#ffff00",
	},
	temporalEnums.WORKFLOW_EXECUTION_STATUS_CONTINUED_AS_NEW.String(): {
		displayName: "Continued as New",
		icon:        "🔄",
		color:       "#800080",
	},
}

// ========================================
// Screen Constants
// ========================================
const (
	// App Header Height
	HEADER_HEIGHT = 2
	// App Search Input Height
	SEARCH_INPUT_HEIGHT = 1
)

// ========================================
// Search Logic and Rendering
// ========================================
var textInputWrapperStyle = lipgloss.NewStyle().Height(SEARCH_INPUT_HEIGHT)

type retrievedSearchOptionsMsg struct {
	searchOptions []string
}

// The struct that stores the search options by serach mode
type activeSearchParams map[searchMode][]string

func (m model) handleSearchModeSelect(msg tea.KeyMsg) model {
	if msg.String() == "w" {
		m.searchMode = WORKFLOWTYPE
		m.searchInput.Prompt = "Search WorkflowType: "
		m.searchInput.Focus()
	}
	if msg.String() == "i" {
		m.searchMode = WORKFLOWID
		m.searchInput.Focus()
		m.searchInput.Prompt = "Search WorkflowId: "
	}
	if msg.String() == "s" {
		m.searchMode = EXECUTIONSTATUS
		m.searchInput.Prompt = "Search WorkflowStatus: "
		m.searchInput.Focus()
	}
	return m
}

func (m model) handleSearchUpdate(msg tea.KeyMsg) (model, tea.Cmd) {
	if m.searchInput.Focused() && msg.String() == "enter" {
		if m.searchMode == EXECUTIONSTATUS {
			caser := cases.Title(language.English)
			m.searchInput.SetValue(caser.String(m.searchInput.Value()))
		}
		m.activeSearchParams[m.searchMode] = append(m.activeSearchParams[m.searchMode], m.searchInput.Value())
		m.searchInput.Blur()
		m.searchInput.SetValue("")
		// Clear searchMode
		m.searchMode = ""
		return m, m.refetchWorkflowsCmd()
	}
	if msg.String() == "esc" {
		m.searchInput.Blur()
		m.searchMode = ""
		return m, nil
	}
	if msg.String() == "ctrl+c" {
		m.searchInput.Blur()
		m.searchMode = ""
		return m, nil
	}
	if m.searchInput.Focused() {
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, tea.Batch(m.getPossibleSearchOptionsCmd, cmd)
	}
	return m, nil
}

func (m model) getPossibleSearchOptionsCmd() tea.Msg {
	if m.searchInput.Value() == "" {
		return []string{}
	}
	if m.searchMode == WORKFLOWTYPE || m.searchMode == WORKFLOWID {
		temporalClient, _ := getTemporalClient()
		query := fmt.Sprintf("%s BETWEEN \"%s\" AND \"%s~\"", m.searchMode, m.searchInput.Value(), m.searchInput.Value())
		result, err := temporalClient.ListWorkflow(context.Background(), &workflowservice.ListWorkflowExecutionsRequest{
			Query:    query,
			PageSize: 1,
		})
		if err != nil {
			log.Fatalf("Failed to list workflows: %v", err)
		}
		opts := []string{}
		for _, w := range result.GetExecutions() {
			if m.searchMode == WORKFLOWID {
				opts = append(opts, w.GetExecution().WorkflowId)
			}
			if m.searchMode == WORKFLOWTYPE {
				opts = append(opts, w.GetType().Name)
			}
		}
		return retrievedSearchOptionsMsg{searchOptions: opts}
	}
	if m.searchMode == EXECUTIONSTATUS {
		opts := []string{
			temporalEnums.WORKFLOW_EXECUTION_STATUS_COMPLETED.String(),
			temporalEnums.WORKFLOW_EXECUTION_STATUS_FAILED.String(),
			temporalEnums.WORKFLOW_EXECUTION_STATUS_CANCELED.String(),
			temporalEnums.WORKFLOW_EXECUTION_STATUS_RUNNING.String(),
			temporalEnums.WORKFLOW_EXECUTION_STATUS_TERMINATED.String(),
			temporalEnums.WORKFLOW_EXECUTION_STATUS_CONTINUED_AS_NEW.String(),
		}
		return retrievedSearchOptionsMsg{searchOptions: opts}

	}
	return []string{}
}

func (m model) constructQueryString() string {
	currentSearchParams := m.activeSearchParams
	// Loop through the search params and construct the query string
	queryString := ""
	for searchMode, searchValues := range currentSearchParams {
		if len(searchValues) == 0 {
			continue
		}
		if queryString != "" {
			queryString += " AND "
		}

		querySegments := []string{}
		for _, searchValue := range searchValues {
			querySegments = append(querySegments, fmt.Sprintf("%s = '%s'", searchMode, searchValue))
		}
		queryGroupString := fmt.Sprintf("(%s)", strings.Join(querySegments, " OR "))
		queryString += queryGroupString
	}
	return queryString
}

func (m model) renderFooter() string {
	if m.searchMode == "" {
		return ""
	}
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
	totalRunning := m.upToDateWorkflowCount[temporalEnums.WORKFLOW_EXECUTION_STATUS_RUNNING]
	totalCompleted := m.upToDateWorkflowCount[temporalEnums.WORKFLOW_EXECUTION_STATUS_COMPLETED]
	headerStyle := lipgloss.NewStyle().Padding(0, 0).Width(m.viewport.Width).Height(HEADER_HEIGHT)
	queryStringStyle := lipgloss.NewStyle().Padding(0, 0).Width(m.viewport.Width).Height(1)
	countStyle := lipgloss.NewStyle().Padding(0, 0).Width(m.viewport.Width).Height(1)
	countStr := fmt.Sprintf("Total Running: %d Total Completed: %d", totalRunning, totalCompleted)
	currentQuery := m.constructQueryString()
	return headerStyle.Render(countStyle.Render(countStr) + "\n" + queryStringStyle.Render(currentQuery))
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
		statusIcon := statusToStyleMap[w.GetStatus().String()].icon
		t.Row(statusIcon, w.GetType().Name, workflowId, startTime, closeTime)
	}
	return tableSurroundStyle.Render(t.Render())
}

// https://github.com/achannarasappa/ticker/blob/master/internal/ui/ui.go#L64

type backgroundUpdateWorkflowCountMsg struct {
	executionStatus temporalEnums.WorkflowExecutionStatus
	count           int64
}

func (m model) backgroundUpdateWorkflowCountCmd(exeuctionStatus temporalEnums.WorkflowExecutionStatus) tea.Cmd {
	return tea.Tick(time.Second*5, func(_ time.Time) tea.Msg {
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
		query := m.constructQueryString()
		queryResult, err := temporalClient.ListWorkflow(context.Background(), &workflowservice.ListWorkflowExecutionsRequest{
			Query:    query,
			PageSize: 40,
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
	return tea.Tick(time.Second*5, func(_ time.Time) tea.Msg {
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
		query := m.constructQueryString()
		if query == "" {
			query = statusQuery
		}
		if query != "" {
			query = fmt.Sprintf("%s AND %s", query, statusQuery)
		}
		queryResult, err := temporalClient.CountWorkflow(context.Background(), &workflowservice.CountWorkflowExecutionsRequest{
			Query: query,
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
	WORKFLOWTYPE    searchMode = "WorkflowType"
	WORKFLOWID      searchMode = "WorkflowId"
	EXECUTIONSTATUS searchMode = "ExecutionStatus"
)

type model struct {
	activeSearchParams         activeSearchParams
	searchMode                 searchMode
	searchOptions              []string
	searchInput                textinput.Model
	ready                      bool
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
	textInput.Prompt = ""
	textInput.ShowSuggestions = true
	activeSearchParams := make(map[searchMode][]string)
	activeSearchParams[WORKFLOWTYPE] = []string{}
	activeSearchParams[WORKFLOWID] = []string{}
	activeSearchParams[EXECUTIONSTATUS] = []string{}
	return model{
		activeSearchParams: activeSearchParams,
		searchInput:        textInput,
		ready:              false,
		workflows:          []*workflow.WorkflowExecutionInfo{},
		selected:           make(map[int]struct{}),
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
	case retrievedSearchOptionsMsg:
		m.searchInput.SetSuggestions(msg.searchOptions)
		m.searchOptions = msg.searchOptions
		return m, nil

	case updateWorkflowsMsg:
		m.workflows = msg.workflows
		return m, nil

	// Is it a key press?
	case tea.KeyMsg:
		if m.searchInput.Focused() {
			return m.handleSearchUpdate(msg)
		}

		m = m.handleSearchModeSelect(msg)

		switch msg.String() {
		// Reset the search params if c is pressed
		case "c":
			m.activeSearchParams = make(map[searchMode][]string)
			return m, m.refetchWorkflowsCmd()
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
		m.backgroundUpdateWorkflowCountCmd(temporalEnums.WORKFLOW_EXECUTION_STATUS_RUNNING),
		m.backgroundUpdateWorkflowCountCmd(temporalEnums.WORKFLOW_EXECUTION_STATUS_FAILED),
		m.backgroundUpdateWorkflowCountCmd(temporalEnums.WORKFLOW_EXECUTION_STATUS_CANCELED),
		m.refetchWorkflowCountCmd(temporalEnums.WORKFLOW_EXECUTION_STATUS_COMPLETED),
		m.refetchWorkflowCountCmd(temporalEnums.WORKFLOW_EXECUTION_STATUS_FAILED),
		m.refetchWorkflowCountCmd(temporalEnums.WORKFLOW_EXECUTION_STATUS_CANCELED),
		m.refetchWorkflowCountCmd(temporalEnums.WORKFLOW_EXECUTION_STATUS_RUNNING),
	)
}
func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
