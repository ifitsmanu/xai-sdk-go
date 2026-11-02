package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ifitsmanu/xai-sdk-go"
	"github.com/ifitsmanu/xai-sdk-go/agent"
	"github.com/ifitsmanu/xai-sdk-go/models"
	"github.com/ifitsmanu/xai-sdk-go/tool"
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
	// The schema for the arguments comes from the struct.
	clock, err := tool.Func("current_time", "Current time in a time zone.",
		func(_ context.Context, in struct {
			Zone string `json:"zone" jsonschema:"IANA time zone, such as America/Chicago"`
		}) (string, error) {
			loc, err := time.LoadLocation(in.Zone)
			if err != nil {
				return "", err
			}
			return time.Now().In(loc).Format(time.RFC1123), nil
		})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	assistant := &agent.Agent{
		Name:        "assistant",
		Model:       model,
		Instruction: "You are a concise assistant. Use tools when they help.",
		Tools:       []tool.Tool{clock, tool.WebSearch{}},
	}

	session := agent.NewSession()
	for ev, err := range client.Runner().Run(context.Background(), session, assistant, "What time is it in Tokyo, and what is the top xAI news today?") {
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
