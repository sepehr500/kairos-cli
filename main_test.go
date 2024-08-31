package main

import (
	"context"
	"testing"
)

// TestHelloName calls greetings.Hello with a name, checking
// for a valid return value.

func TestHelloName(t *testing.T) {
	println("HELLO")
	client, _ := getTemporalClient()
	something, _ := client.DescribeWorkflowExecution(context.Background(), "qaiTasksWorkflow-a66a6572-5fff-4298-8732-98fe64f945a8", "1098cba9-b906-4b9a-b60d-57eec943bedc")
	pending := something.GetPendingActivities()
	for _, activity := range pending {
		println(activity.GetAttempt())
		println(activity.GetLastFailure().String())
	}
	// history := client.GetWorkflowHistory(context.Background(), "cityworksImport-62795e06-87e7-4b9d-9403-a0460acf4909-workflow-2024-08-21T04:05:00Z", "53ffe192-0150-4111-b26b-09f919579176", false, enums.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	// i := 0
	// for history.HasNext() {
	// 	event, err := history.Next()
	// 	attributes := event.GetActivityTaskStartedEventAttributes().GetAttempt()
	// 	println(attributes)
	// 	if err != nil {
	// 		break
	// 	}
	// }
}
