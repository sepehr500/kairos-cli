package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"sync"

	"github.com/BurntSushi/toml"
	"go.temporal.io/api/history/v1"
	"go.temporal.io/api/workflow/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	tlog "go.temporal.io/sdk/log"
)

var (
	temporalClient client.Client
	once           sync.Once
)

type (
	// Config struct
	TomlConfig struct {
		Namespace map[string]struct {
			TemporalCloudHost  string `toml:"temporal_cloud_host"`
			TemporalNamespace  string `toml:"temporal_namespace"`
			TemporalPrivateKey string `toml:"temporal_private_key"`
			TemporalPublicKey  string `toml:"temporal_public_key"`
		} `toml:"namespace"`
	}
)

func getTemporalClient() (client.Client, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("Error fetching home directory:", err)
	}
	var config TomlConfig
	f := filepath.Join(homeDir, ".kairos", "credentials")
	if _, err := os.Stat(f); err != nil {
	}
	if _, err := toml.DecodeFile(f, &config); err != nil {
		log.Fatal(err)
	}

	cert, err := tls.X509KeyPair([]byte(config.Namespace["default"].TemporalPublicKey), []byte(config.Namespace["default"].TemporalPrivateKey))
	if err != nil {
		log.Fatalf("Failed to load Temporal credentials: %v", err)
	}

	once.Do(func() {
		clientOptions := client.Options{
			Namespace: config.Namespace["default"].TemporalNamespace,
			HostPort:  config.Namespace["default"].TemporalCloudHost,
			ConnectionOptions: client.ConnectionOptions{
				TLS: &tls.Config{
					Certificates: []tls.Certificate{
						cert,
					},
				},
			},

			Logger: tlog.NewStructuredLogger(
				slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
					AddSource: true,
					Level:     slog.LevelDebug,
				}))),
		}
		var err error
		temporalClient, err = client.Dial(clientOptions)
		if err != nil {
			log.Fatalf("Failed to create Temporal client: %v", err)
		}
	})
	return temporalClient, nil
}

func listWorkflows(workflowName string) []*workflow.WorkflowExecutionInfo {

	temporalClient, _ := getTemporalClient()

	query := fmt.Sprintf("WorkflowType BETWEEN \"%s\" AND \"%s~\" AND CloseTime is null", workflowName, workflowName)
	if workflowName == "" {
		query = "CloseTime is null"
	}
	result, err := temporalClient.ListWorkflow(context.Background(), &workflowservice.ListWorkflowExecutionsRequest{
		Query:    query,
		PageSize: 20,
	})
	if err != nil {
		log.Fatalf("Failed to list workflows: %v", err)
	}

	return result.GetExecutions()
}

func KickoffWorkflow(workflowName string, payload string) (string, error) {
	temporalClient, _ := getTemporalClient()
	options := client.StartWorkflowOptions{
		ID:        workflowName,
		TaskQueue: "general",
	}
	var convertedPayload map[string]interface{}
	err := json.Unmarshal([]byte(payload), &convertedPayload)
	if err != nil {
		return "", err
	}

	we, err := temporalClient.ExecuteWorkflow(context.Background(), options, workflowName, convertedPayload)

	if err != nil {
		return "", err
	}

	return we.GetRunID(), nil
}

func GetWorkflowHistory(workflowID string, runID string) ([]*history.HistoryEvent, error) {
	temporalClient, _ := getTemporalClient()
	defer temporalClient.Close()
	historyList := temporalClient.GetWorkflowHistory(context.Background(), workflowID, runID, false, 0)

	events := []*history.HistoryEvent{}
	for historyList.HasNext() {
		historyEvent, err := historyList.Next()
		println(historyEvent.String())
		if err != nil {
			return []*history.HistoryEvent{}, err
		}
		events = append(events, historyEvent)
	}

	return events, nil
}
