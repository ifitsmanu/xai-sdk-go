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
	"github.com/ifitsmanu/xai-sdk-go/models"
	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
)

func main() {
	client, err := xai.NewClient()
	if err != nil {
		panic(err)
	}
	defer client.Close()

	// The newest language model this key can call. models.Need also filters
	// by context size, price, reasoning effort, and image input.
	model, err := client.Models.Select(context.Background(), models.Need{})
	if err != nil {
		panic(err)
	}

	conversation := client.Chat.Create(chat.Config{
		Model: model,
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

`client.Chat`, `client.Imagine`, `client.Files`, `client.Models`, and `client.Batch` are separate APIs on that connection. `client.Imagine.Image` and `client.Imagine.Video` are creation. `client.Auth`, `client.Tokenize`, `client.Embed`, `client.Documents`, and `client.Sample` are the smaller ones.

`Chat.Defer` returns a request id. `Chat.Await` polls that deferred completion for up to ten minutes. `Sample.Stream` streams the low-level sampler. Batch requests and one result are `client.Batch.Requests` and `client.Batch.Result`.

These are not gRPC methods, so this client calls them over HTTP or a WebSocket. `client.Responses` is the Responses API, including `Dial` for the `/v1/responses` socket. `client.Skills` and `client.Account` are the other inference REST calls. `client.Voice` is speech, transcription, and `wss://…/v1/realtime`. `WithHost(xai.HostUS)` and `WithHost(xai.HostMTLS)` plus `WithClientCertificate` select the documented hosts. `client.Management` is set from `XAI_MANAGEMENT_API_KEY` and covers collections, billing, audit, and team API keys. Search in a collection stays on `client.Documents`.

Declare tools once with package `tool`. Functions run in your process; web search, X search, code execution, collections, image generation, and remote MCP run at xAI. `tool.Func` builds the argument schema from a Go struct and validates calls against it. The same list converts to each API, whose wire shapes differ:

```go
weather, err := tool.Func("weather", "Weather for a city.",
	func(ctx context.Context, in struct {
		City string `json:"city" jsonschema:"city name"`
	}) (string, error) {
		return lookup(ctx, in.City)
	})

tools := []tool.Tool{weather, tool.WebSearch{}, tool.CodeExecution{}, tool.MCP{Label: "docs", URL: serverURL}}
chatTools, err := tool.ForChat(tools)           // chat.Config.Tools
respTools, err := tool.ForResponses(tools)      // responses.Request.Tools
voiceTools, err := tool.ForVoice(tools)         // voice.Session.Tools
```

An agent takes the list directly as `Agent.Tools`.

Image models come from gRPC. Video models come from `GET /v1/video-generation-models`, because that list is not a gRPC method. `client.Models.All` is `GET /v1/models`.

```go
images, err := client.Models.ImageNames(ctx)
videos, err := client.Models.VideoNames(ctx)
client.Imagine.Image.Generate(ctx, &xaiapi.GenerateImageRequest{Model: images[0], Prompt: "a red cube"})
client.Imagine.Video.Generate(ctx, &xaiapi.GenerateVideoRequest{Model: videos[0], Prompt: "a red cube spinning"})
```

Set `Image` to `image.URL` or `image.File` to edit. On a video request, `Image` is the first frame, `Video` edits, `ReferenceImages` is reference-to-video, and `LastFrame` and `Keyframes` pin frames. Set `StorageOptions` on either request to keep the file. `batch.Image`, `batch.Video`, and `batch.Completion` wrap those requests for `Batch.AddRequests`. `Chat.SampleN` asks for several completions. `Chat.Compact` replaces the transcript with the compaction result.

## Agent

An agent is an instruction, a model, and tools. `client.Runner()` calls the model, runs the tool, and stores the transcript on a session.

```go
assistant := &agent.Agent{
	Name:        "assistant",
	Model:       model,
	Instruction: "You are concise. Use tools when they help.",
	Tools:       []tool.Tool{tool.WebSearch{}},
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
