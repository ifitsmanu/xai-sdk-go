# xai-sdk-go

Go client for the [xAI API](https://docs.x.ai).

Written by [Manu](https://github.com/ifitsmanu). This is not an xAI product. The official SDK is [xai-sdk-python](https://github.com/xai-org/xai-sdk-python). The messages are generated from [xai-proto](https://github.com/xai-org/xai-proto).

```bash
go get github.com/ifitsmanu/xai-sdk-go
```

Set `XAI_API_KEY`. Calls go to `api.x.ai`.

## Chat

```go
package main

import (
	"context"
	"fmt"

	"github.com/ifitsmanu/xai-sdk-go"
	"github.com/ifitsmanu/xai-sdk-go/chat"
	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
)

func main() {
	client, err := xai.NewClient()
	if err != nil {
		panic(err)
	}
	defer client.Close()

	conversation := client.Chat.Create(chat.Config{
		Model: "grok-4",
		Messages: []*xaiapi.Message{
			chat.SystemText("You are concise."),
		},
	})
	conversation.Append(chat.UserText("Hello"))

	resp, err := conversation.Sample(context.Background())
	if err != nil {
		panic(err)
	}
	fmt.Println(resp.Content())
}
```

`client.Image`, `client.Video`, `client.Files`, `client.Models`, `client.Batch`, `client.Auth`, `client.Tokenize`, `client.Embed`, `client.Documents`, and `client.Sample` are the other APIs.

## Agent

An agent is an instruction, a model, and tools. `client.Runner()` calls the model, runs the tool, and stores the transcript on a session.

```go
assistant := &agent.Agent{
	Name:        "assistant",
	Model:       "grok-4",
	Instruction: "You are concise. Use tools when they help.",
}

session := agent.NewSession()
for ev, err := range client.Runner().Run(ctx, session, assistant, "What time is it?") {
	if err != nil {
		return err
	}
	if ev.Kind == agent.KindText {
		fmt.Println(ev.Text)
	}
}
```

A full tool example is in `examples/agent`.

## Try it

From a clone:

```bash
make ui      # terminal chat
make chat    # one completion
make agent   # one tool call
```

`make ui` opens with no key. Set `XAI_API_KEY` before you send. Logs stay off unless you pass `xai.WithLogLevel`. The key is not written to the log.

## License

Apache-2.0. Development commands, pull requests, and version tags are in [CONTRIBUTING.md](CONTRIBUTING.md).
