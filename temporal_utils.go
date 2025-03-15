package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"sync"

	"github.com/BurntSushi/toml"
	"go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
	tlog "go.temporal.io/sdk/log"
)

var (
	temporalClient client.Client
	namespace      string
	once           sync.Once
	configOnce     sync.Once
)

type NamespaceInfo struct {
	TemporalCloudHost  string `toml:"temporal_cloud_host"`
	TemporalNamespace  string `toml:"temporal_namespace"`
	TemporalPrivateKey string `toml:"temporal_private_key"`
	TemporalPublicKey  string `toml:"temporal_public_key"`
}

type (
	// Config struct
	TomlConfig struct {
		Namespace map[string]NamespaceInfo `toml:"namespace"`
	}
)

func (m model) getTemporalConfig() NamespaceInfo {
	isLocal := flag.Bool("local", false, "Connect to local temporal on localhost:7233")
	configOnce.Do(func() {
		namespace = *flag.String("namespace", "default", "Namespace")
		if *isLocal {
			namespace = "default"
		}
		flag.Parse()
	})
	if *isLocal == true {
		return NamespaceInfo{
			TemporalCloudHost:  "localhost:7233",
			TemporalNamespace:  "default",
			TemporalPrivateKey: "",
			TemporalPublicKey:  "",
		}
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("Error fetching home directory:", err)
	}
	var config TomlConfig
	f := filepath.Join(homeDir, ".config", "kairos", "credentials")
	if _, err := os.Stat(f); err != nil {
	}
	if _, err := toml.DecodeFile(f, &config); err != nil {
		log.Fatal("Temporal credentials are missing. Please add credentials to .config/kairos/credentials")
		os.Exit(0)
	}

	return config.Namespace[namespace]

}

func (m model) getTemporalClient() (client.Client, error) {

	once.Do(func() {
		config := m.getTemporalConfig()
		flag.Parse()
		var clientOptions client.Options
		if strings.Contains(config.TemporalCloudHost, "localhost") {
			clientOptions =
				client.Options{
					HostPort: config.TemporalCloudHost,
					Logger: tlog.NewStructuredLogger(
						slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
							AddSource: true,
							Level:     slog.LevelDebug,
						}))),
				}
		} else {
			cert, err := tls.X509KeyPair([]byte(config.TemporalPublicKey), []byte(config.TemporalPrivateKey))
			if err != nil {
				log.Fatalf("Failed to load Temporal credentials: %v", err)
			}
			clientOptions = client.Options{
				Namespace: config.TemporalNamespace,
				HostPort:  config.TemporalCloudHost,
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
		}
		var err error
		temporalClient, err = client.Dial(clientOptions)
		if err != nil {
			log.Fatalf("Failed to create Temporal client: %v", err)
		}
	})
	return temporalClient, nil
}

func (m *model) openWorkflowInBrowser(workflowID string, runID string) {
	config := m.getTemporalConfig()
	host := "https://cloud.temporal.io"
	if strings.Contains(config.TemporalCloudHost, "localhost") {
		host = "http://localhost:8233"
	}
	url := host + "/namespaces/" + config.TemporalNamespace + "/workflows/" + workflowID + "/" + runID + "/history"
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		return
	}

	cmd.Stdout = nil
	cmd.Stderr = nil

	err := cmd.Start()

	if err != nil {
		log.Fatalf("Failed to open browser: %v", err)
	}

}

func (m model) KickoffWorkflow(workflowName string, payload string) (string, error) {
	temporalClient, _ := m.getTemporalClient()
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

func (m model) GetWorkflowHistory(workflowID string, runID string) ([]*history.HistoryEvent, error) {
	temporalClient, _ := m.getTemporalClient()
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
