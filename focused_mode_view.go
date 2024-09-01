package main

import (
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	temporalEnums "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/history/v1"
)

var leftBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
var rightBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
var bottomBoxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder())

type compactHistoryListItem struct {
	events     []*history.HistoryEvent
	icon       string
	actionType string
	rowContent string
}

type compactedHistory map[int64]*compactHistoryListItem

func createCompactHistory(historyList []*history.HistoryEvent) compactedHistory {
	compactedHistory := make(compactedHistory)
	for _, historyEvent := range historyList {
		switch historyEvent.GetEventType() {
		// Activity events
		case temporalEnums.EVENT_TYPE_ACTIVITY_TASK_SCHEDULED:
			eventId := historyEvent.GetEventId()
			compactedHistory[eventId] = &compactHistoryListItem{events: make([]*history.HistoryEvent, 0)}
			compactedHistory[eventId].actionType = "Activity"
			compactedHistory[eventId].icon = "📅"
			compactedHistory[eventId].rowContent = historyEvent.GetActivityTaskScheduledEventAttributes().GetActivityType().GetName()
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
			event.icon = "✅"
			event.events = append(event.events, historyEvent)
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
			compactedHistory[eventId].icon = "👶🏃"
			compactedHistory[eventId].events = append(compactedHistory[childWorkflowExecutionStartedEventAttributes.GetInitiatedEventId()].events, historyEvent)

		case temporalEnums.EVENT_TYPE_CHILD_WORKFLOW_EXECUTION_COMPLETED:
			childWorkflowExecutionCompletedEventAttributes := historyEvent.GetChildWorkflowExecutionCompletedEventAttributes()
			eventId := childWorkflowExecutionCompletedEventAttributes.GetInitiatedEventId()
			compactedHistory[eventId].icon = "👶✅"
			compactedHistory[eventId].events = append(compactedHistory[childWorkflowExecutionCompletedEventAttributes.GetInitiatedEventId()].events, historyEvent)
		case temporalEnums.EVENT_TYPE_CHILD_WORKFLOW_EXECUTION_FAILED:
			childWorkflowExecutionFailedEventAttributes := historyEvent.GetChildWorkflowExecutionFailedEventAttributes()
			eventId := childWorkflowExecutionFailedEventAttributes.GetInitiatedEventId()
			compactedHistory[eventId].icon = "👶❌"
			compactedHistory[eventId].events = append(compactedHistory[childWorkflowExecutionFailedEventAttributes.GetInitiatedEventId()].events, historyEvent)
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
	compactHistory := createCompactHistory(selectedWorkflow.history)
	historyEventTableStyle := table.New().
		Border(lipgloss.NormalBorder())
	// Convert compactHistory into a slice
	compactHistorySlice := make([]*compactHistoryListItem, 0)
	for _, compactHistoryItem := range compactHistory {
		compactHistorySlice = append(compactHistorySlice, compactHistoryItem)
	}

	sort.Slice(compactHistorySlice, func(i, j int) bool {
		// Sort by the first eventid
		return compactHistorySlice[i].events[0].GetEventId() < compactHistorySlice[j].events[0].GetEventId()
	})
	for _, compactHistoryItem := range compactHistorySlice {
		firstEvent := compactHistoryItem.events[0]
		historyEventTableStyle.Row(compactHistoryItem.icon, strconv.FormatInt(firstEvent.GetEventId(), 10), compactHistoryItem.actionType, compactHistoryItem.rowContent)
	}
	return lipgloss.JoinVertical(lipgloss.Left, lipgloss.JoinHorizontal(lipgloss.Left, leftBoxStyle.Render(selectedWorkflow.workflow.String()), rightBoxStyle.Render("TEST")), bottomBoxStyle.Render(historyEventTableStyle.Render()))

}
