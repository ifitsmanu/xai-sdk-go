package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/ifitsmanu/xai-sdk-go"
	"github.com/ifitsmanu/xai-sdk-go/agent"
	"github.com/ifitsmanu/xai-sdk-go/models"
)

func main() {
	client, err := xai.NewClient(xai.WithLogLevel(xai.LevelDebug))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer func() {
		if err := client.Close(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()

	model, err := client.Models.Select(context.Background(), models.Need{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	assistant := &agent.Agent{
		Name:        "assistant",
		Model:       model,
		Instruction: "You are a concise assistant. Use tools when they help.",
		Tools: []agent.Tool{
			agent.Func("current_time", "Current UTC time.", map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}, func(context.Context, json.RawMessage) (string, error) {
				return time.Now().UTC().Format(time.RFC3339), nil
			}),
		},
	}

	session := agent.NewSession()
	for ev, err := range client.Runner().Run(context.Background(), session, assistant, "What time is it?") {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		switch ev.Kind {
		case agent.KindToolCall:
			fmt.Printf("tool %s %s\n", ev.Tool, ev.Args)
		case agent.KindToolResult:
			fmt.Printf("result %s\n", ev.Result)
		case agent.KindText:
			fmt.Println(ev.Text)
		}
	}
}
