package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"go.temporal.io/api/common/v1"
	temporalEnums "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/history/v1"
	"go.temporal.io/api/workflow/v1"
	"go.temporal.io/api/workflowservice/v1"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func formatNumber(number int) string {
	switch {
	case number > 1_000_000:
		return fmt.Sprintf("%.1fM", float64(number)/1_000_000)
	case number > 1_000:
		return fmt.Sprintf("%.1fK", float64(number)/1_000)
	default:
		return fmt.Sprintf("%d", number)
	}
}

// ========================================
// Keybindings
// ========================================

type KeyMap struct {
	Up                       key.Binding
	Down                     key.Binding
	SearchWorkflowType       key.Binding
	SearchWorkflowId         key.Binding
	SearchExecutionStatus    key.Binding
	Help                     key.Binding
	Exit                     key.Binding
	ClearSearch              key.Binding
	RefetchWorkflows         key.Binding
	Select                   key.Binding
	OpenWorkflowInWeb        key.Binding
	TerminateWorkflow        key.Binding
	RestartWorkflow          key.Binding
	ToggleParentWorkflowMode key.Binding
	FocusWorkflow            key.Binding
	NextPage                 key.Binding
	PrevPage                 key.Binding
}

var DefaultKeyMap = KeyMap{
	Up: key.NewBinding(
		key.WithKeys("k", "up"),        // actual keybindings
		key.WithHelp("↑/k", "move up"), // corresponding help text
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("↓/j", "move down"),
	),
	SearchWorkflowType: key.NewBinding(
		key.WithKeys("w", "search Type"),
		key.WithHelp("w", "search Type"),
	),
	SearchWorkflowId: key.NewBinding(
		key.WithKeys("i", "search WorkflowId"),
		key.WithHelp("i", "search WorkflowId"),
	),
	SearchExecutionStatus: key.NewBinding(
		key.WithKeys("s", "search Status"),
		key.WithHelp("s", "search Status"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
	Exit: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "exit"),
	),
	ClearSearch: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "clear search"),
	),
	RefetchWorkflows: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refetch"),
	),
	Select: key.NewBinding(
		key.WithKeys("enter", "space"),
		key.WithHelp("enter/space", "toggle selection"),
	),
	OpenWorkflowInWeb: key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "open in web"),
	),
	TerminateWorkflow: key.NewBinding(
		key.WithKeys("t"),
		key.WithHelp("t", "terminate workflow"),
	),
	RestartWorkflow: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "restart workflow"),
	),
	ToggleParentWorkflowMode: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "toggle parent workflow mode"),
	),
	FocusWorkflow: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "focus workflow"),
	),
	NextPage: key.NewBinding(
		key.WithKeys("]"),
		key.WithHelp("]", "Go to next page"),
	),
	PrevPage: key.NewBinding(
		key.WithKeys("["),
		key.WithHelp("[", "Go to previous page"),
	),
}

// ShortHelp returns keybindings to be shown in the mini help view. It's part
// of the key.Map interface.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.SearchWorkflowType, k.SearchWorkflowId, k.SearchExecutionStatus, k.Help, k.ClearSearch, k.RefetchWorkflows,
		k.Select, k.OpenWorkflowInWeb, k.TerminateWorkflow, k.RestartWorkflow, k.Exit,
	}
}

// FullHelp returns keybindings for the expanded help view. It's part of the
// key.Map interface.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.SearchWorkflowType, k.SearchExecutionStatus, k.SearchWorkflowId, k.ToggleParentWorkflowMode, k.OpenWorkflowInWeb, k.ClearSearch, k.RefetchWorkflows, k.RestartWorkflow, k.TerminateWorkflow, k.Exit, k.NextPage, k.PrevPage},
	}
}

// ========================================
// Status Styles
// ========================================

// Status String to style map

var temporalEnumStatusList = []string{
	temporalEnums.WORKFLOW_EXECUTION_STATUS_RUNNING.String(),
	temporalEnums.WORKFLOW_EXECUTION_STATUS_COMPLETED.String(),
	temporalEnums.WORKFLOW_EXECUTION_STATUS_FAILED.String(),
	temporalEnums.WORKFLOW_EXECUTION_STATUS_CANCELED.String(),
	temporalEnums.WORKFLOW_EXECUTION_STATUS_TERMINATED.String(),
	// I removed the CONTINUED_AS_NEW status
}

var TABLE_LIST_PAGE_SIZE = 40

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
		color:       "#0000ff",
	},
	temporalEnums.WORKFLOW_EXECUTION_STATUS_TERMINATED.String(): {
		displayName: "Terminated",
		// Skull icon
		icon:  "💀",
		color: "#ffff00",
	},
	temporalEnums.WORKFLOW_EXECUTION_STATUS_CONTINUED_AS_NEW.String(): {
		displayName: "Cont. New",
		icon:        "🔄",
		color:       "#800080",
	},
}

// ========================================
// Confirmation Message Flow
// ========================================

type confirmationFlowStateEnums string

const (
	NO_FLOW_RUNNING       confirmationFlowStateEnums = "NO_FLOW_RUNNING"
	AWAITING_CONFIRMATION confirmationFlowStateEnums = "AWAITING_CONFIRMATION"
	EXECUTING_ACTION      confirmationFlowStateEnums = "EXECUTING_ACTION"
	ACTION_COMPLETED      confirmationFlowStateEnums = "ACTION_COMPLETED"
)

type confirmationFlowStateMsg struct {
	state                         confirmationFlowStateEnums
	pendingConfirmationMessage    string
	executionSuccessMessage       string
	areYouSureMessage             string
	commandThatRunsOnConfirmation tea.Cmd
}

func (m model) startConfirmationMessageFlowCmd(confirmationFlowStateMsg confirmationFlowStateMsg) tea.Cmd {
	return func() tea.Msg {
		confirmationFlowStateMsg.state = AWAITING_CONFIRMATION
		return func() tea.Msg {
			confirmationFlowStateMsg.commandThatRunsOnConfirmation()
			confirmationFlowStateMsg.state = ACTION_COMPLETED
			return confirmationFlowStateMsg
		}
	}
}

// ========================================
// Screen Constants
// ========================================
const (
	// App Header Height
	HEADER_HEIGHT = 4
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
	if key.Matches(msg, m.keys.SearchWorkflowType) {
		m.searchMode = WORKFLOWTYPE
		m.searchInput.Prompt = "Search WorkflowType: "
		m.searchInput.Focus()
	}
	if key.Matches(msg, m.keys.SearchWorkflowId) {
		m.searchMode = WORKFLOWID
		m.searchInput.Focus()
		m.searchInput.Prompt = "Search WorkflowId: "
	}
	if key.Matches(msg, m.keys.SearchExecutionStatus) {
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
		m.searchMode = ""
		m.clearListState()
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
		temporalClient, _ := m.getTemporalClient()
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
	if m.parentWorkflowMode {
		queryString = "ParentWorkflowId is null"
	}
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
	if m.confirmationFlowState.state == EXECUTING_ACTION {
		return m.confirmationFlowState.pendingConfirmationMessage + "..."
	}
	if m.confirmationFlowState.state == ACTION_COMPLETED {
		return m.confirmationFlowState.executionSuccessMessage
	}
	if m.confirmationFlowState.state == AWAITING_CONFIRMATION {
		return m.confirmationFlowState.areYouSureMessage + " (y/n)"
	}
	helpView := m.help.View(m.keys)
	if m.searchMode == "" {
		return helpView
	}
	textInputWrapperStyle := textInputWrapperStyle.Width(m.viewport.Width)
	searchInputStyle := m.searchInput.View()
	return textInputWrapperStyle.Render(searchInputStyle)

}

// ========================================
// Table Logic and Rendering
// ========================================

type setFocusedWorkflowMsg struct {
	compactedHistoryStackItem compactHistoryStackItem
}

func (m *model) clearListState() {
	m.page = 0
	m.cursor = 0
	nextPageTokenCache := make(map[int][]byte)
	nextPageTokenCache[0] = []byte{}
	m.nextPageTokenCache = nextPageTokenCache
}

func (m *model) setFocusedWorkflowCmd(workflowId string, runId string) tea.Cmd {
	return func() tea.Msg {
		temporalClient, _ := m.getTemporalClient()
		executionDescription, err := temporalClient.DescribeWorkflowExecution(context.Background(), workflowId, runId)
		historyIterator := temporalClient.GetWorkflowHistory(context.Background(), workflowId, runId, false, 0)

		pendingActivities := executionDescription.GetPendingActivities()
		// Nested loop. We break out of the loop if we find an activity with an attempt > 0
		// The append below will alows run
		if err != nil {
			log.Fatalf("Failed to describe workflow: %v", err)
		}
		history := []*history.HistoryEvent{}
		for historyIterator.HasNext() {
			historyEvent, err := historyIterator.Next()
			if err != nil {
				log.Fatalf("Failed to get workflow history: %v", err)
			}
			history = append(history, historyEvent)
		}
		compactedHistory := createCompactHistory(history, pendingActivities)
		newCompactedHistoryStackItem := compactHistoryStackItem{
			workflowId:          workflowId,
			runId:               runId,
			compactHistory:      compactedHistory,
			workflowDescription: executionDescription,
		}
		return setFocusedWorkflowMsg{compactedHistoryStackItem: newCompactedHistoryStackItem}
	}
}

func (m model) clearCompletionCmd() tea.Cmd {
	return tea.Tick(time.Second*3, func(_ time.Time) tea.Msg {
		m.confirmationFlowState.state = NO_FLOW_RUNNING
		return m.confirmationFlowState
	})

}

func (m model) restartWorkflowCmd(workflowId string, runId string) tea.Cmd {
	restartWorkflowCmd := func() tea.Msg {
		temporalClient, _ := m.getTemporalClient()
		namespaceInfo := m.getTemporalConfig()
		workflowHistory := temporalClient.GetWorkflowHistory(context.Background(), workflowId, runId, false, 0)
		// Find first eventId that is  `WORKFLOW_TASK_COMPLETED`,`WORKFLOW_TASK_TIMED_OUT`, `WORKFLOW_TASK_FAILED`
		eventId := int64(0)
		for workflowHistory.HasNext() {
			historyEvent, err := workflowHistory.Next()
			if err != nil {
				log.Fatalf("Failed to get workflow history: %v", err)
			}
			switch historyEvent.GetEventType() {
			case temporalEnums.EVENT_TYPE_WORKFLOW_TASK_COMPLETED, temporalEnums.EVENT_TYPE_WORKFLOW_TASK_TIMED_OUT, temporalEnums.EVENT_TYPE_WORKFLOW_TASK_FAILED:
				eventId = historyEvent.GetEventId()
				break
			}
		}

		namespace := namespaceInfo.TemporalNamespace
		if eventId == 0 {
			log.Fatalf("Failed to find eventId to restart workflow")
		}
		_, err := temporalClient.ResetWorkflowExecution(context.Background(),
			&workflowservice.ResetWorkflowExecutionRequest{
				Namespace: namespace,
				WorkflowExecution: &common.WorkflowExecution{
					WorkflowId: workflowId,
					RunId:      runId,
				},
				Reason:                    "CLI Restart",
				WorkflowTaskFinishEventId: eventId,
			},
		)
		if err != nil {
			log.Fatalf("Failed to restart workflow: %v", err)
		}
		return nil
	}
	return func() tea.Msg {
		return confirmationFlowStateMsg{
			state:                         AWAITING_CONFIRMATION,
			executionSuccessMessage:       "Workflow restarted successfully",
			areYouSureMessage:             fmt.Sprintf("Are you sure you want to restart workflow %s?", workflowId),
			pendingConfirmationMessage:    "Are you sure you want to restart this workflow?",
			commandThatRunsOnConfirmation: restartWorkflowCmd,
		}
	}
}

func (m model) terminateWorkflowCmd(workflowId string, runId string) tea.Cmd {
	termanateWorkflowCmd := func() tea.Msg {
		temporalClient, _ := m.getTemporalClient()
		err := temporalClient.TerminateWorkflow(context.Background(), workflowId, runId, "CLI Termination")
		if err != nil {
			log.Fatalf("Failed to terminate workflow: %v", err)
		}
		return nil
	}
	return func() tea.Msg {
		return confirmationFlowStateMsg{
			state:                         AWAITING_CONFIRMATION,
			areYouSureMessage:             fmt.Sprintf("Are you sure you want to terminate workflow %s?", workflowId),
			pendingConfirmationMessage:    "Are you sure you want to terminate this workflow?",
			commandThatRunsOnConfirmation: termanateWorkflowCmd,
		}
	}
}

func (m model) renderHeader() string {
	headerStyle := lipgloss.NewStyle().Padding(0, 0).Width(m.viewport.Width).Height(HEADER_HEIGHT)
	queryStringStyle := lipgloss.NewStyle().Padding(0, 0).Width(m.viewport.Width).Height(1)
	// Construct the count string
	currentQuery := m.constructQueryString()
	// Order the upToDateWorkflowCount map by the order of the temporalEnums
	// This is to ensure that the order of the counts is consistent

	styleStrArray := []string{}
	for _, status := range temporalEnumStatusList {
		upperCaseStatus := strings.ToUpper(status)
		statusInt := temporalEnums.WorkflowExecutionStatus_value[fmt.Sprintf("WORKFLOW_EXECUTION_STATUS_%s", upperCaseStatus)]
		count := m.upToDateWorkflowCount[temporalEnums.WorkflowExecutionStatus(statusInt)]
		currentCountStyle := lipgloss.NewStyle()
		styleStruct := statusToStyleMap[status]
		rawCountStr := fmt.Sprintf("%s %s: %s ", styleStruct.icon, styleStruct.displayName, formatNumber(int(count)))
		renderedStr := currentCountStyle.Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(styleStruct.color)).Render(rawCountStr)
		styleStrArray = append(styleStrArray, renderedStr)
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, styleStrArray...)

	return headerStyle.Render(row + "\n" + queryStringStyle.Render(currentQuery))
}

var HeaderStyle = lipgloss.NewStyle().Padding(0, 0).Bold(true)
var EvenRowStyle = lipgloss.NewStyle().Padding(0, 0).Background(lipgloss.Color("#222222"))
var OddRowStyle = lipgloss.NewStyle().Padding(0, 0)
var SelectedRowStyle = lipgloss.NewStyle().Padding(0, 0).Background(lipgloss.Color("#005500"))

var highlightedStatusIconStyle = lipgloss.NewStyle().Background(lipgloss.Color("#0000ff")).Foreground(lipgloss.Color("#ffffff"))

var attemptsStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000"))

func (m model) renderTable(workflows []*workflowTableListItem) string {
	helpHeight := lipgloss.Height(m.help.View(m.keys))
	tableSurroundStyle := lipgloss.NewStyle().Padding(0, 0).Height(m.viewport.Height - HEADER_HEIGHT - helpHeight)
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderRight(false).
		BorderLeft(false).
		BorderTop(false).
		BorderBottom(false).
		BorderHeader(true).
		BorderColumn(true).
		Width(m.viewport.Width).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == m.cursor:
				return SelectedRowStyle
			case row%2 == 0:
				return EvenRowStyle
			default:
				return OddRowStyle
			}
		}).
		Headers("Status", "Type", "Id", "Start Time", "Close Time", "Attempts")
	for _, w := range workflows {
		workflowId := w.workflow.Execution.WorkflowId
		closeTime := w.workflow.GetCloseTime().AsTime().In(time.Local).Format(time.RFC3339)
		// If close time starts with 1970, it means the workflow is still running and has no close time
		if w.workflow.GetStatus().String() == "Running" {
			closeTime = "--"
		}
		attempts := strconv.Itoa(int(w.attempts))
		if w.attempts > 3 {
			attempts = attemptsStyle.Render(attempts)
		}
		if w.attempts == 0 {
			attempts = "--"
		}
		childIcon := ""
		if w.workflow.GetParentExecution().GetWorkflowId() != "" {
			childIcon = "👶"
		}
		statusIcon := statusToStyleMap[w.workflow.GetStatus().String()].icon
		startTimeDiff := getRelativeTimeDiff(time.Now(), w.workflow.GetStartTime().AsTime())

		t.Row(statusIcon+childIcon, w.workflow.GetType().Name, workflowId, startTimeDiff, closeTime, attempts)
	}
	return tableSurroundStyle.Render(t.Render())
}

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
	workflows     []*workflowTableListItem
	nextPageToken []byte
}

type refetchWorkflowCmdOptions struct {
	nextPageToken []byte
}

func (m *model) refetchWorkflowsCmd() tea.Cmd {
	return func() tea.Msg {
		temporalClient, _ := m.getTemporalClient()
		query := m.constructQueryString()
		nextPageToken := m.nextPageTokenCache[m.page]
		queryResult, err := temporalClient.ListWorkflow(context.Background(), &workflowservice.ListWorkflowExecutionsRequest{
			Query:         query,
			PageSize:      int32(TABLE_LIST_PAGE_SIZE),
			NextPageToken: nextPageToken,
		})
		if err != nil {
			log.Fatalf("Failed to list workflows: %v", err)
		}
		result := queryResult.GetExecutions()
		returnObj := []*workflowTableListItem{}
		for _, workflow := range result {
			listItem := &workflowTableListItem{workflow: workflow, attempts: 0}
			returnObj = append(returnObj, listItem)
		}
		// I don't love this...but it's needed to solve the issue of  updateVisibleWorkflowAttempsBackgroundCmd
		// Need the workflow list be up to date. tea.Sequence runs when the message is returned, not when the message is handled
		// TODO: Restructure code so updateVisibleWorkflowAttempsBackgroundCmd runs after the updateWorkflowsMsg is handled
		m.workflows = returnObj
		return updateWorkflowsMsg{workflows: returnObj, nextPageToken: queryResult.NextPageToken}
	}
}

type updateVisibleWorkflowsMsg struct {
	workflowsMap map[string]*workflow.WorkflowExecutionInfo
}

type updateVisibleWorkflowAttempsMsg struct {
	updateMapping map[string]int32
}

func (m *model) updateVisibleWorkflowAttempsBackgroundCmd(delay time.Duration) tea.Cmd {
	return tea.Tick(time.Second*delay, func(_ time.Time) tea.Msg {
		returnObj := make(map[string]int32)
		temporalClient, _ := m.getTemporalClient()
		currentRunningExecutionIds := []string{}
		for _, execution := range m.workflows {
			if execution.workflow.GetCloseTime() == nil {
				currentRunningExecutionIds = append(currentRunningExecutionIds, "'"+execution.workflow.GetExecution().WorkflowId+"'")
			}
		}
		if len(currentRunningExecutionIds) == 0 {
			return updateVisibleWorkflowAttempsMsg{updateMapping: returnObj}
		}
		query := fmt.Sprintf("WorkflowId IN (%s)", strings.Join(currentRunningExecutionIds, ","))
		queryResult, err := temporalClient.ListWorkflow(context.Background(), &workflowservice.ListWorkflowExecutionsRequest{
			Query:    query,
			PageSize: int32(TABLE_LIST_PAGE_SIZE),
		})
		if err != nil {
			log.Fatalf("Failed to list workflows: %v", err)
		}
		// Look for workflows that are in the current list and update them
		workflows := queryResult.GetExecutions()
		for _, workflow := range workflows {
			// Skip if started less that 10 minutes ago
			if workflow.GetStartTime().AsTime().UTC().After(time.Now().UTC().Add(-10 * time.Minute)) {
				continue
			}
			if workflow.GetStatus() == temporalEnums.WORKFLOW_EXECUTION_STATUS_RUNNING {
				execution, err := temporalClient.DescribeWorkflowExecution(
					context.Background(),
					workflow.GetExecution().WorkflowId,
					workflow.GetExecution().RunId,
				)
				if err != nil {
					break
				}

				pendingActivities := execution.GetPendingActivities()
				// Nested loop. We break out of the loop if we find an activity with an attempt > 0
				for _, activity := range pendingActivities {
					for _, currentWorkflow := range m.workflows {
						if workflow.GetExecution().WorkflowId == currentWorkflow.workflow.GetExecution().WorkflowId {
							returnObj[workflow.GetExecution().WorkflowId] = activity.GetAttempt()
							break
						}
					}
				}
				continue
			}
		}
		return updateVisibleWorkflowAttempsMsg{updateMapping: returnObj}
	})
}

func (m model) updateVisibleWorkflowsBackgroundCmd() tea.Cmd {
	return tea.Tick(time.Second*5, func(_ time.Time) tea.Msg {
		returnObj := make(map[string]*workflow.WorkflowExecutionInfo)
		temporalClient, _ := m.getTemporalClient()
		currentRunningExecutionIds := []string{}
		for _, execution := range m.workflows {
			if execution.workflow.GetCloseTime() == nil {
				currentRunningExecutionIds = append(currentRunningExecutionIds, "'"+execution.workflow.GetExecution().WorkflowId+"'")
			}
		}
		if len(currentRunningExecutionIds) == 0 {
			return updateVisibleWorkflowsMsg{workflowsMap: returnObj}
		}
		query := fmt.Sprintf("WorkflowId IN (%s)", strings.Join(currentRunningExecutionIds, ","))
		queryResult, err := temporalClient.ListWorkflow(context.Background(), &workflowservice.ListWorkflowExecutionsRequest{
			Query:    query,
			PageSize: int32(TABLE_LIST_PAGE_SIZE),
		})
		if err != nil {
			log.Fatalf("Failed to list workflows: %v", err)
		}
		// Look for workflows that are in the current list and update them
		workflows := queryResult.GetExecutions()
		for _, updatedWorkflow := range workflows {
			for _, currentWorkflow := range m.workflows {
				if updatedWorkflow.GetExecution().WorkflowId == currentWorkflow.workflow.GetExecution().WorkflowId {
					returnObj[updatedWorkflow.GetExecution().WorkflowId] = updatedWorkflow
				}
			}
		}

		return updateVisibleWorkflowsMsg{workflowsMap: returnObj}
	})
}

type updateWorkflowCountMsg struct {
	executionStatus temporalEnums.WorkflowExecutionStatus
	count           int64
}

func (m model) refetchWorkflowCountCmd(executionStatus temporalEnums.WorkflowExecutionStatus) tea.Cmd {
	return func() tea.Msg {
		temporalClient, _ := m.getTemporalClient()
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

type workflowTableListItem struct {
	workflow          *workflow.WorkflowExecutionInfo
	history           []*history.HistoryEvent
	pendingActivities []*workflow.PendingActivityInfo
	attempts          int32
}

type model struct {
	focusedWorkflowState  focusedModeState
	parentWorkflowMode    bool
	confirmationFlowState confirmationFlowStateMsg
	keys                  KeyMap
	help                  help.Model
	page                  int
	nextPageTokenCache    map[int][]byte
	activeSearchParams    activeSearchParams
	searchMode            searchMode
	searchOptions         []string
	searchInput           textinput.Model
	ready                 bool
	workflows             []*workflowTableListItem
	cursor                int // which to-do list item our cursor is pointing at
	selected              map[int]bool
	viewport              viewport.Model
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
	nextPageTokenCache := make(map[int][]byte)
	nextPageTokenCache[0] = []byte{}
	return model{
		nextPageTokenCache: nextPageTokenCache,
		page:               0,
		focusedWorkflowState: focusedModeState{
			keys:                  FocusedModeKeyMap,
			compactedHistoryStack: make([]compactHistoryStackItem, 0),
		},
		parentWorkflowMode: false,
		confirmationFlowState: confirmationFlowStateMsg{
			state:                         NO_FLOW_RUNNING,
			pendingConfirmationMessage:    "",
			areYouSureMessage:             "",
			executionSuccessMessage:       "",
			commandThatRunsOnConfirmation: func() tea.Msg { return nil },
		},
		cursor:             0,
		keys:               DefaultKeyMap,
		help:               help.New(),
		activeSearchParams: activeSearchParams,
		searchInput:        textInput,
		ready:              false,
		workflows:          []*workflowTableListItem{},
		selected:           make(map[int]bool),
		upToDateWorkflowCount: map[temporalEnums.WorkflowExecutionStatus]int64{
			temporalEnums.WORKFLOW_EXECUTION_STATUS_COMPLETED: 0,
			temporalEnums.WORKFLOW_EXECUTION_STATUS_RUNNING:   0,
			temporalEnums.WORKFLOW_EXECUTION_STATUS_FAILED:    0,
			temporalEnums.WORKFLOW_EXECUTION_STATUS_CANCELED:  0,
		},
	}
}

func (m model) View() string {
	if len(m.focusedWorkflowState.compactedHistoryStack) > 0 {
		return m.focusedModeView()
	}
	view := m.renderHeader() + "\n" + m.renderTable(m.workflows) + "\n" + m.renderFooter()
	return view
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.help.Width = msg.Width
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height)
			m.viewport.YPosition = 0
			m.ready = true

		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height

		}

	case setFocusedWorkflowMsg:
		m.focusedWorkflowState.cursor = 0
		m.focusedWorkflowState.compactedHistoryStack = append(m.focusedWorkflowState.compactedHistoryStack, msg.compactedHistoryStackItem)
		return m, nil

	case confirmationFlowStateMsg:
		m.confirmationFlowState = msg
		switch msg.state {
		case NO_FLOW_RUNNING, AWAITING_CONFIRMATION, EXECUTING_ACTION:
			m.confirmationFlowState = msg
			return m, nil
		case ACTION_COMPLETED:
			m.confirmationFlowState = msg
			m.clearListState()
			return m, m.clearCompletionCmd()
		}
		return m, nil

	case updateWorkflowCountMsg:
		m.upToDateWorkflowCount[msg.executionStatus] = msg.count
		return m, nil
	case backgroundUpdateWorkflowCountMsg:
		m.upToDateWorkflowCount[msg.executionStatus] = msg.count
		return m, m.backgroundUpdateWorkflowCountCmd(msg.executionStatus)

	case updateVisibleWorkflowAttempsMsg:
		for i, existingWorkflow := range m.workflows {
			workflowId := existingWorkflow.workflow.GetExecution().WorkflowId
			if _, ok := msg.updateMapping[workflowId]; ok {
				m.workflows[i].attempts = msg.updateMapping[workflowId]
			}
		}
		return m, m.updateVisibleWorkflowAttempsBackgroundCmd(10)

	case updateVisibleWorkflowsMsg:
		for i, existingWorkflow := range m.workflows {
			workflowId := existingWorkflow.workflow.GetExecution().WorkflowId
			if _, ok := msg.workflowsMap[workflowId]; ok {
				m.workflows[i].workflow = msg.workflowsMap[workflowId]
			}
		}
		return m, m.updateVisibleWorkflowsBackgroundCmd()
	case retrievedSearchOptionsMsg:
		m.searchInput.SetSuggestions(msg.searchOptions)
		m.searchOptions = msg.searchOptions
		return m, nil

	case updateWorkflowsMsg:
		m.workflows = msg.workflows
		m.nextPageTokenCache[m.page+1] = msg.nextPageToken
		return m, nil

	// Is it a key press?
	case tea.KeyMsg:
		if m.confirmationFlowState.state == AWAITING_CONFIRMATION {
			if msg.String() == "y" {
				m.confirmationFlowState.state = EXECUTING_ACTION
				// Wrap the command to set the state to action completed
				wrappedFunc := func() tea.Msg {
					m.confirmationFlowState.commandThatRunsOnConfirmation()
					m.confirmationFlowState.state = ACTION_COMPLETED
					m.clearListState()
					return m.confirmationFlowState
				}
				return m, wrappedFunc
			}
			if msg.String() == "n" {
				m.confirmationFlowState.state = NO_FLOW_RUNNING
				return m, nil
			}
		}
		if m.searchInput.Focused() {
			return m.handleSearchUpdate(msg)
		}

		m = m.handleSearchModeSelect(msg)

		switch {
		// These keys should exit the program.
		case key.Matches(msg, m.keys.Exit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.ToggleParentWorkflowMode):
			m.parentWorkflowMode = !m.parentWorkflowMode
			return m, m.refetchWorkflowsCmd()
		case key.Matches(msg, m.keys.RestartWorkflow):
			if m.cursor < len(m.workflows) {
				workflowId := m.workflows[m.cursor].workflow.GetExecution().WorkflowId
				runId := m.workflows[m.cursor].workflow.Execution.GetRunId()
				return m, m.restartWorkflowCmd(workflowId, runId)
			}

		case key.Matches(msg, m.keys.TerminateWorkflow):
			if m.cursor < len(m.workflows) {
				workflowId := m.workflows[m.cursor].workflow.GetExecution().WorkflowId
				runId := m.workflows[m.cursor].workflow.Execution.GetRunId()
				return m, m.terminateWorkflowCmd(workflowId, runId)
			}
		case key.Matches(msg, m.keys.OpenWorkflowInWeb):
			if m.cursor < len(m.workflows) {
				workflowId := m.workflows[m.cursor].workflow.GetExecution().WorkflowId
				runId := m.workflows[m.cursor].workflow.Execution.GetRunId()
				m.openWorkflowInBrowser(workflowId, runId)
			}
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		case key.Matches(msg, m.keys.NextPage):
			m.page++
			return m, m.refetchWorkflowsCmd()
		case key.Matches(msg, m.keys.PrevPage):
			if m.page > 0 {
				m.page--
			}
			return m, m.refetchWorkflowsCmd()
		// Reset the search params if c is pressed
		case key.Matches(msg, m.keys.ClearSearch):
			m.activeSearchParams = make(map[searchMode][]string)
			m.cursor = 0
			m.page = 0
			return m, m.refetchWorkflowsCmd()
		case key.Matches(msg, m.keys.RefetchWorkflows):
			return m, m.refetchWorkflowsCmd()
			// The "enter" key and the spacebar (a literal space) toggle
			// the selected state for the item that the cursor is pointing at.
		case len(m.focusedWorkflowState.compactedHistoryStack) > 0:
			return m.UpdateFocusedModeState(msg)

		case key.Matches(msg, m.keys.FocusWorkflow):
			if m.cursor < len(m.workflows) {
				currentWorkflow := m.workflows[m.cursor]
				return m, m.setFocusedWorkflowCmd(currentWorkflow.workflow.Execution.WorkflowId, currentWorkflow.workflow.Execution.RunId)
			}
			return m, nil

		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}

		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.workflows)-1 {
				m.cursor++
			}
		case key.Matches(msg, m.keys.Select):
			if m.cursor < len(m.workflows) {
				m.selected[m.cursor] = true
			}
		}
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return m, nil
}

func (m model) Init() tea.Cmd {
	return tea.Sequence(
		m.refetchWorkflowsCmd(),
		tea.Batch(
			m.updateVisibleWorkflowsBackgroundCmd(),
			m.backgroundUpdateWorkflowCountCmd(temporalEnums.WORKFLOW_EXECUTION_STATUS_COMPLETED),
			m.backgroundUpdateWorkflowCountCmd(temporalEnums.WORKFLOW_EXECUTION_STATUS_RUNNING),
			m.backgroundUpdateWorkflowCountCmd(temporalEnums.WORKFLOW_EXECUTION_STATUS_FAILED),
			m.backgroundUpdateWorkflowCountCmd(temporalEnums.WORKFLOW_EXECUTION_STATUS_TERMINATED),
			m.refetchWorkflowCountCmd(temporalEnums.WORKFLOW_EXECUTION_STATUS_COMPLETED),
			m.refetchWorkflowCountCmd(temporalEnums.WORKFLOW_EXECUTION_STATUS_FAILED),
			m.refetchWorkflowCountCmd(temporalEnums.WORKFLOW_EXECUTION_STATUS_TERMINATED),
			m.refetchWorkflowCountCmd(temporalEnums.WORKFLOW_EXECUTION_STATUS_RUNNING),
			m.updateVisibleWorkflowAttempsBackgroundCmd(3),
		),
	)
}
func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
