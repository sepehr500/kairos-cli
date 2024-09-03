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
	workflowIdStack []string
	focusedWorkflow *workflowTableListItem
	cursor          int
	keys            FocusedKeyMap
}

func (m *model) UpdateFocusedModeState(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.focusedWorkflowState.keys.Up):
			m.focusedWorkflowState.cursor--
		case key.Matches(msg, m.focusedWorkflowState.keys.Down):
			m.focusedWorkflowState.cursor++
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
		switch historyEvent.GetEventType() {
		// Activity events
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
			prettyJSONString := convertDataToPrettyJSON(attributes.GetInput().GetPayloads()[0].GetData())
			compactedHistory[eventId].events = append(compactedHistory[eventId].events, historyEvent)
			compactedHistory[eventId].eventsContent = append(compactedHistory[eventId].eventsContent, prettyJSONString)
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

// Each border is .5 characters wide, so we subtract 2 from the width and height
func (m model) focusedModeView() string {
	selectedWorkflow := m.focusedWorkflowState.focusedWorkflow

	boxWidth := m.viewport.Width / 2
	// boxHeight := m.viewport.Height * 2 / 3
	// leftBoxStyle := leftBoxStyle.Width(boxWidth).Height(boxHeight).Padding(0, 0).Margin(0, 0)
	// rightBoxStyle := rightBoxStyle.Width(boxWidth-x*2).Height(boxHeight).Padding(0, 0).Margin(0, 0)
	historyListBoxStyleWithDem := historyListBoxStyle.Height(m.viewport.Height - 2).Width(boxWidth - 2)
	historyDetailBoxStyleWithDem := historyDetailBoxStyle.Height(m.viewport.Height - 2).Width(boxWidth - 2)

	// Bottom box

	compactHistory := createCompactHistory(selectedWorkflow.history, selectedWorkflow.pendingActivities)

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

	focusedHistoryEvents := compactHistorySlice[m.focusedWorkflowState.cursor].eventsContent
	focusedHistoryEventContent := ""
	for _, historyEvent := range focusedHistoryEvents {
		focusedHistoryEventContent += historyEvent + "\n"
	}

	return lipgloss.JoinHorizontal(lipgloss.Center, historyDetailBoxStyleWithDem.Render(focusedHistoryEventContent), historyListBoxStyleWithDem.Render(historyEventTableStyle.Render()))

}
