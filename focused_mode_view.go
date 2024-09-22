package main

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	temporalEnums "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/history/v1"
	"go.temporal.io/api/workflow/v1"
)

type FocusedKeyMap struct {
	Up   key.Binding
	Down key.Binding
	Exit key.Binding
	Back key.Binding
}

var FocusedModeKeyMap = FocusedKeyMap{
	Up: key.NewBinding(
		key.WithKeys("k", "up"),        // actual keybindings
		key.WithHelp("↑/k", "move up"), // corresponding help text
	),
	Down: key.NewBinding(
		key.WithKeys("j", "down"),
		key.WithHelp("↓/j", "move down"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "esc"),
	),
	Exit: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "exit"),
	),
}

type focusedModeState struct {
	workflowIdStack  []string
	focusedWorkflow  *workflowTableListItem
	cursor           int
	keys             FocusedKeyMap
	compactedHistory compactedHistory
}

func (m *model) UpdateFocusedModeState(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.focusedWorkflowState.keys.Up):
			if m.focusedWorkflowState.cursor > 0 {
				m.focusedWorkflowState.cursor--
			}
		case key.Matches(msg, m.focusedWorkflowState.keys.Down):
			if m.focusedWorkflowState.cursor < len(m.focusedWorkflowState.compactedHistory)-1 {
				m.focusedWorkflowState.cursor++
			}
		case key.Matches(msg, m.focusedWorkflowState.keys.Back):
			m.focusedWorkflowState.focusedWorkflow = nil
			m.focusedWorkflowState.cursor = 0
			m.focusedWorkflowState.workflowIdStack = []string{}
		case key.Matches(msg, m.focusedWorkflowState.keys.Exit):
			return m, tea.Quit
		}

	}
	return m, nil
}

type compactHistoryListItem struct {
	events        []*history.HistoryEvent
	eventsContent []string
	icon          string
	actionType    string
	rowContent    string
}

type compactedHistory map[int64]*compactHistoryListItem

var activityNameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF00FF")).Bold(true)
var jsonOutputStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF00FF")).Bold(false)

func convertDataToPrettyJSON(data []byte) string {
	var prettyJSON map[string]interface{}
	json.Unmarshal(data, &prettyJSON)
	prettyJSONBytes, _ := json.MarshalIndent(prettyJSON, "", "  ")
	return string(prettyJSONBytes)
}

func createCompactHistory(historyList []*history.HistoryEvent, pendingActivities []*workflow.PendingActivityInfo) compactedHistory {
	compactedHistory := make(compactedHistory)
	for _, historyEvent := range historyList {

		eventType := historyEvent.GetEventType()
		switch historyEvent.GetEventType() {
		// Activity events
		// Activity events are special because they have multiple events that are related to each other
		// Activity events are grouped by the scheduled event id
		case temporalEnums.EVENT_TYPE_ACTIVITY_TASK_SCHEDULED:
			eventId := historyEvent.GetEventId()
			attributes := historyEvent.GetActivityTaskScheduledEventAttributes()
			compactedHistory[eventId] = &compactHistoryListItem{events: make([]*history.HistoryEvent, 0)}
			compactedHistory[eventId].actionType = "Activity"
			compactedHistory[eventId].icon = "📅"

			compactedHistory[eventId].rowContent = historyEvent.GetActivityTaskScheduledEventAttributes().GetActivityType().GetName()
			for _, pendingActivity := range pendingActivities {
				if pendingActivity.GetActivityId() == attributes.GetActivityId() {
					compactedHistory[eventId].rowContent += " 🔄" + strconv.Itoa(int(pendingActivity.GetAttempt()))
					break
				}
			}
			if attributes.GetInput().GetPayloads() != nil {
				prettyJSONString := convertDataToPrettyJSON(attributes.GetInput().GetPayloads()[0].GetData())
				compactedHistory[eventId].eventsContent = append(compactedHistory[eventId].eventsContent, prettyJSONString)
			}
			compactedHistory[eventId].events = append(compactedHistory[eventId].events, historyEvent)
		case temporalEnums.EVENT_TYPE_ACTIVITY_TASK_STARTED:
			activityTaskStartedEventAttributes := historyEvent.GetActivityTaskStartedEventAttributes()
			eventId := activityTaskStartedEventAttributes.GetScheduledEventId()
			compactedHistory[eventId].icon = "🏃"
			compactedHistory[eventId].events = append(compactedHistory[activityTaskStartedEventAttributes.GetScheduledEventId()].events, historyEvent)
		case temporalEnums.EVENT_TYPE_ACTIVITY_TASK_COMPLETED:
			activityTaskCompletedEventAttributes := historyEvent.GetActivityTaskCompletedEventAttributes()
			eventId := activityTaskCompletedEventAttributes.GetScheduledEventId()
			event := compactedHistory[eventId]
			prettyJsonString := convertDataToPrettyJSON(activityTaskCompletedEventAttributes.GetResult().GetPayloads()[0].GetData())
			event.icon = "✅"
			event.events = append(event.events, historyEvent)
			compactedHistory[eventId].eventsContent = append(compactedHistory[eventId].eventsContent, prettyJsonString)
		case temporalEnums.EVENT_TYPE_ACTIVITY_TASK_FAILED:
			activityTaskFailedEventAttributes := historyEvent.GetActivityTaskFailedEventAttributes()
			eventId := activityTaskFailedEventAttributes.GetScheduledEventId()
			compactedHistory[eventId].icon = "❌"
			compactedHistory[activityTaskFailedEventAttributes.GetScheduledEventId()].events = append(compactedHistory[activityTaskFailedEventAttributes.GetScheduledEventId()].events, historyEvent)
		case temporalEnums.EVENT_TYPE_ACTIVITY_TASK_TIMED_OUT:
			activityTaskTimedOutEventAttributes := historyEvent.GetActivityTaskTimedOutEventAttributes()
			eventId := activityTaskTimedOutEventAttributes.GetScheduledEventId()
			compactedHistory[eventId].icon = "⏰"
			compactedHistory[activityTaskTimedOutEventAttributes.GetScheduledEventId()].events = append(compactedHistory[activityTaskTimedOutEventAttributes.GetScheduledEventId()].events, historyEvent)
		case temporalEnums.EVENT_TYPE_ACTIVITY_TASK_CANCEL_REQUESTED:
			activityTaskCancelRequestedEventAttributes := historyEvent.GetActivityTaskCancelRequestedEventAttributes()
			eventId := activityTaskCancelRequestedEventAttributes.GetScheduledEventId()
			compactedHistory[eventId].icon = "🚫"
			compactedHistory[activityTaskCancelRequestedEventAttributes.GetScheduledEventId()].events = append(compactedHistory[activityTaskCancelRequestedEventAttributes.GetScheduledEventId()].events, historyEvent)
		case temporalEnums.EVENT_TYPE_ACTIVITY_TASK_CANCELED:
			activityTaskCanceledEventAttributes := historyEvent.GetActivityTaskCanceledEventAttributes()
			eventId := activityTaskCanceledEventAttributes.GetScheduledEventId()
			compactedHistory[eventId].icon = "🚫"
			compactedHistory[activityTaskCanceledEventAttributes.GetScheduledEventId()].events = append(compactedHistory[activityTaskCanceledEventAttributes.GetScheduledEventId()].events, historyEvent)
		// Timer events
		case temporalEnums.EVENT_TYPE_TIMER_STARTED:
			eventId := historyEvent.GetEventId()
			// initialize the compacted history list
			compactedHistory[eventId] = &compactHistoryListItem{events: make([]*history.HistoryEvent, 0)}
			compactedHistory[eventId].actionType = "Timer"
			compactedHistory[eventId].icon = "⏰"
			compactedHistory[eventId].rowContent = historyEvent.GetTimerStartedEventAttributes().GetTimerId()
			compactedHistory[eventId].events = append(compactedHistory[eventId].events, historyEvent)
		case temporalEnums.EVENT_TYPE_TIMER_FIRED:
			timerFiredEventAttributes := historyEvent.GetTimerFiredEventAttributes()
			eventId := timerFiredEventAttributes.GetStartedEventId()
			compactedHistory[eventId].icon = "🔥"
			compactedHistory[eventId].events = append(compactedHistory[timerFiredEventAttributes.GetStartedEventId()].events, historyEvent)
		case temporalEnums.EVENT_TYPE_TIMER_CANCELED:
			timerCanceledEventAttributes := historyEvent.GetTimerCanceledEventAttributes()
			eventId := timerCanceledEventAttributes.GetStartedEventId()
			compactedHistory[eventId].icon = "🚫"
			compactedHistory[eventId].events = append(compactedHistory[timerCanceledEventAttributes.GetStartedEventId()].events, historyEvent)

		// Child workflow events
		case temporalEnums.EVENT_TYPE_START_CHILD_WORKFLOW_EXECUTION_INITIATED:
			eventId := historyEvent.GetEventId()
			// initialize the compacted history list
			compactedHistory[eventId] = &compactHistoryListItem{events: make([]*history.HistoryEvent, 0)}
			compactedHistory[eventId].actionType = "Child Workflow"
			compactedHistory[eventId].icon = "👶🏃"
			compactedHistory[eventId].rowContent = historyEvent.GetStartChildWorkflowExecutionInitiatedEventAttributes().GetWorkflowType().GetName()

		case temporalEnums.EVENT_TYPE_CHILD_WORKFLOW_EXECUTION_STARTED:
			childWorkflowExecutionStartedEventAttributes := historyEvent.GetChildWorkflowExecutionStartedEventAttributes()
			eventId := childWorkflowExecutionStartedEventAttributes.GetInitiatedEventId()
			compactedHistory[eventId].icon = "🏃👶"
			compactedHistory[eventId].events = append(compactedHistory[childWorkflowExecutionStartedEventAttributes.GetInitiatedEventId()].events, historyEvent)

		case temporalEnums.EVENT_TYPE_CHILD_WORKFLOW_EXECUTION_COMPLETED:
			childWorkflowExecutionCompletedEventAttributes := historyEvent.GetChildWorkflowExecutionCompletedEventAttributes()
			eventId := childWorkflowExecutionCompletedEventAttributes.GetInitiatedEventId()
			compactedHistory[eventId].icon = "✅👶"
			compactedHistory[eventId].events = append(compactedHistory[childWorkflowExecutionCompletedEventAttributes.GetInitiatedEventId()].events, historyEvent)
		case temporalEnums.EVENT_TYPE_CHILD_WORKFLOW_EXECUTION_FAILED:
			childWorkflowExecutionFailedEventAttributes := historyEvent.GetChildWorkflowExecutionFailedEventAttributes()
			eventId := childWorkflowExecutionFailedEventAttributes.GetInitiatedEventId()
			compactedHistory[eventId].icon = "❌👶"
		// General workflow events
		case temporalEnums.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED:
			eventId := historyEvent.GetEventId()
			executionStartedEventAttributes := historyEvent.GetWorkflowExecutionStartedEventAttributes()

			// initialize the compacted history list
			compactedHistory[eventId] = &compactHistoryListItem{events: make([]*history.HistoryEvent, 0)}
			inputPayloads := executionStartedEventAttributes.GetInput().GetPayloads()

			if inputPayloads != nil {
				prettyJsonString := convertDataToPrettyJSON(executionStartedEventAttributes.GetInput().GetPayloads()[0].GetData())
				compactedHistory[eventId].eventsContent = append(compactedHistory[eventId].eventsContent, prettyJsonString)
			}
			compactedHistory[eventId].actionType = eventType.String()
			compactedHistory[eventId].icon = "🚀"
			compactedHistory[eventId].rowContent = "Workflow started"
			compactedHistory[eventId].events = append(compactedHistory[eventId].events, historyEvent)
		case temporalEnums.EVENT_TYPE_WORKFLOW_EXECUTION_COMPLETED:
			eventId := historyEvent.GetEventId()
			compactedHistory[eventId] = &compactHistoryListItem{events: make([]*history.HistoryEvent, 0)}
			executionCompletedEventAttributes := historyEvent.GetWorkflowExecutionCompletedEventAttributes()
			outputPayloads := executionCompletedEventAttributes.GetResult().GetPayloads()
			if outputPayloads != nil {
				prettyJsonString := convertDataToPrettyJSON(executionCompletedEventAttributes.GetResult().GetPayloads()[0].GetData())
				compactedHistory[eventId].eventsContent = append(compactedHistory[eventId].eventsContent, prettyJsonString)
			}
			compactedHistory[eventId].actionType = eventType.String()
			compactedHistory[eventId].icon = "✅"
			compactedHistory[eventId].events = append(compactedHistory[eventId].events, historyEvent)

		default:
			eventId := historyEvent.GetEventId()
			eventType := historyEvent.GetEventType()
			// initialize the compacted history list
			if compactedHistory[eventId] == nil && !strings.Contains(eventType.String(), "WorkflowTask") {
				compactedHistory[eventId] = &compactHistoryListItem{events: make([]*history.HistoryEvent, 0)}
				compactedHistory[eventId].actionType = eventType.String()
				compactedHistory[eventId].events = append(compactedHistory[eventId].events, historyEvent)
			}

		}
	}
	return compactedHistory
}

var leftBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
var rightBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
var bottomBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
var historyListBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
var historyDetailBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
var topBarStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())

var (
	topBarHeight = 3
)

func getModuleBorderStyle(width int, title string) lipgloss.Style {
	border := lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "┌",
		TopRight:    "┐",
		BottomLeft:  "└",
		BottomRight: "┘",
	}
	firstPartOfBorder := "--|" + title + "|"
	border.Top = "--|" + title + "|" + strings.Repeat("-", width-len(firstPartOfBorder)-1)

	return lipgloss.NewStyle().
		Border(border).
		Width(width).
		MaxWidth(width + 10)

}

func truncateTextBlock(text string, maxHeight int, maxWidth int) string {
	totalNewLines := strings.Count(text, "\n")
	if totalNewLines < maxHeight {
		return text
	}
	// Change linesliens to H char
	tmpStyle := lipgloss.NewStyle().Width(maxWidth).Render(text)
	newLineIndex := 0
	newLineCount := 0
	for true {
		// Index of the next newline
		tmpNewLineIndex := strings.Index(tmpStyle[newLineIndex:], "\n")
		newLineIndex += tmpNewLineIndex
		if tmpNewLineIndex == -1 {
			break
		}
		if newLineCount == maxHeight {
			break
		}
		newLineCount++
		// Add 1 to the newline index to skip the newline character
		newLineIndex++

	}
	if newLineIndex-3 < 0 {
		return tmpStyle
	}
	return tmpStyle[:newLineIndex-3] + "..."

}

func (m *model) createEventDetailsRows(compactHistoryListItem compactHistoryListItem, width int, height int) string {
	focusedHistoryEvents := compactHistoryListItem.eventsContent
	focusedHistoryEventContent := ""
	for _, historyEvent := range focusedHistoryEvents {
		truncatedHistoryEvent := truncateTextBlock(historyEvent, 10, width)
		focusedHistoryEventContent += getModuleBorderStyle(width-2, "Event Details").Render(truncatedHistoryEvent) + "\n"
	}
	return lipgloss.NewStyle().Width(width).Height(height).Render(focusedHistoryEventContent)
}

// Each border is .5 characters wide, so we subtract 2 from the width and height
func (m model) focusedModeView() string {

	boxWidth := m.viewport.Width / 2
	bottomAreaHeight := m.viewport.Height - topBarHeight - 5
	historyListBoxStyleWithDem := historyListBoxStyle.Height(bottomAreaHeight).Width(boxWidth - 2)
	// historyDetailBoxStyleWithDem := getModuleBorderStyle(boxWidth-2, "Details").MaxHeight(m.viewport.Height - 2 - topBarHeight)

	// Bottom box

	compactHistory := m.focusedWorkflowState.compactedHistory

	historyEventTableStyle := table.New().
		Width(historyListBoxStyleWithDem.GetWidth()).
		Border(lipgloss.HiddenBorder()).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == m.focusedWorkflowState.cursor+1:
				return SelectedRowStyle
			case row == 0:
				return HeaderStyle
			case row%2 == 0:
				return EvenRowStyle
			default:
				return OddRowStyle
			}
		})

	// Convert compactHistory into a slice
	compactHistorySlice := make([]*compactHistoryListItem, 0)

	for _, compactHistoryItem := range compactHistory {
		compactHistorySlice = append(compactHistorySlice, compactHistoryItem)
	}

	// Pending events
	sort.Slice(compactHistorySlice, func(i, j int) bool {
		// Sort by the first eventid
		return compactHistorySlice[i].events[0].GetEventId() > compactHistorySlice[j].events[0].GetEventId()
	})
	for _, compactHistoryItem := range compactHistorySlice {
		firstEvent := compactHistoryItem.events[0]
		historyEventTableStyle.Row(compactHistoryItem.icon, strconv.FormatInt(firstEvent.GetEventId(), 10), compactHistoryItem.actionType, compactHistoryItem.rowContent)
	}

	focusedHistoryEvents := compactHistorySlice[m.focusedWorkflowState.cursor]
	focusedHistoryEventContent := m.createEventDetailsRows(*focusedHistoryEvents, boxWidth-2, bottomAreaHeight)
	statusIcon := statusToStyleMap[m.focusedWorkflowState.focusedWorkflow.workflow.GetStatus().String()].icon
	childIcon := ""
	if m.focusedWorkflowState.focusedWorkflow.workflow.GetParentExecution() != nil {
		childIcon = "👶"
	}
	topBarContent := topBarStyle.Height(topBarHeight - 2).Width(m.viewport.Width - 3).Render(childIcon + " " + statusIcon + " Workflow ID: " + m.focusedWorkflowState.focusedWorkflow.workflow.Execution.GetWorkflowId())

	return lipgloss.JoinVertical(lipgloss.Top, topBarContent, lipgloss.JoinHorizontal(lipgloss.Top, focusedHistoryEventContent, historyListBoxStyleWithDem.Render(historyEventTableStyle.Render())))

}
