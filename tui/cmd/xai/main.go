package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/ifitsmanu/xai-sdk-go"
	"github.com/ifitsmanu/xai-sdk-go/tui"
)

func main() {
	client, sender, hasKey, err := openClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if client != nil {
		defer func() {
			if err := client.Close(); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}()
	}
	names := languageModels(client)
	model := tui.New(tui.Config{
		Sender:    sender,
		HasKey:    hasKey,
		Models:    names,
		ModelName: firstName(names),
	})
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := program.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func openClient() (*xai.Client, tui.Sender, bool, error) {
	if os.Getenv("XAI_API_KEY") == "" {
		return nil, nil, false, nil
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	level := xai.LevelError
	var opts []xai.Option
	if path := os.Getenv("XAI_LOG"); path != "" {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, nil, false, err
		}
		logger = slog.New(slog.NewTextHandler(file, &slog.HandlerOptions{Level: slog.LevelDebug}))
		level = xai.LevelDebug
		if os.Getenv("XAI_LOG_BODY") == "1" {
			opts = append(opts, xai.WithLogBodies())
		}
	}
	opts = append(opts, xai.WithLogger(logger), xai.WithLogLevel(level))
	client, err := xai.NewClient(opts...)
	if err != nil {
		return nil, nil, false, err
	}
	return client, tui.ClientSender{Client: client}, true, nil
}

func languageModels(client *xai.Client) []string {
	if client == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	names, err := client.Models.LanguageNames(ctx)
	if err != nil {
		return nil
	}
	return names
}

func firstName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return names[0]
}
