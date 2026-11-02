package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ifitsmanu/xai-sdk-go"
	"github.com/ifitsmanu/xai-sdk-go/chat"
	"github.com/ifitsmanu/xai-sdk-go/models"
	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
)

func main() {
	client, err := xai.NewClient(xai.WithLogLevel(xai.LevelInfo))
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
	conversation := client.Chat.Create(chat.Config{
		Model: model,
		Messages: []*xaiapi.Message{
			chat.SystemText("You are a concise assistant."),
		},
	})
	conversation.Append(chat.UserText("Say hello in one sentence."))
	resp, err := conversation.Sample(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(resp.Content())
}
