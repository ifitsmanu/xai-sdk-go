# xai-sdk-go

Independent open-source Go client for the [xAI API](https://docs.x.ai). It speaks the public gRPC API at `api.x.ai`, and it includes an agent runtime for tools, sessions, and handoff.

This repository is not published by xAI. The official SDK is [xai-sdk-python](https://github.com/xai-org/xai-sdk-python). Wire messages are generated from the public protobufs in [xai-proto](https://github.com/xai-org/xai-proto).

The module is `github.com/ifitsmanu/xai-sdk-go`. `ifitsmanu` is Manu's GitHub account, and it is only the module path. The API name in code is the xAI API: host `api.x.ai`, key `XAI_API_KEY`, generated package `xaiapi`. The version is `Version` in `doc.go`.

## Commands

From `xai-sdk-go`:

```bash
make            # show this list
make fmt        # format Go code with gofmt
make lint       # check formatting and lint
make test       # run tests
make check      # lint and test; run this before a push
make ui         # open the terminal chat
make chat       # run the chat example
make agent      # run the agent example
make generate   # regenerate code from ../xai-proto
make bench      # time the client and agent hot path
```

`make ui` opens with no key. Set `XAI_API_KEY` and Enter sends. The terminal chat lives in `tui/` and runs from a clone.

`make generate` needs a sibling checkout of `xai-proto`. The first run downloads `buf` into `bin/`.

```text
enter       send
alt+enter   new line
esc         stop
?           help
/model      switch model
/system     set the system prompt
/clear      clear the transcript
/quit       leave
```

## Layout

```
xai-sdk-go/
  client.go            connection, options, log levels
  xaiapi/              generated protobuf and gRPC stubs
  chat/                transcripts, sample, stream, tools
  image/  video/       generation, video polling
  files/               chunked upload and download
  models/  batch/      model catalog and batches
  auth/  tokenize/     key info and token counts
  embed/  documents/   embeddings and document search
  sample/              low-level text sampling
  agent/               tool loop, sessions, handoff, parallel and sequential runs
  internal/rpc/        dial, bearer auth, retry, gRPC logs
  examples/
  tui/                 terminal chat, its own module
```

## Client

```go
client, err := xai.NewClient() // XAI_API_KEY
defer client.Close()

conversation := client.Chat.Create(chat.Config{
    Model: "grok-4",
    Messages: []*xaiapi.Message{chat.SystemText("You are concise.")},
})
conversation.Append(chat.UserText("Hello"))
resp, err := conversation.Sample(ctx)
```

`client.GRPC()` is the underlying `*grpc.ClientConn` when you need a raw stub.

Do not change a chat while `Sample`, `Stream`, or `Defer` is running. Calls that start work are not retried. Reads retry `UNAVAILABLE`.

## Logging

Logs use `log/slog` and stay off unless you opt in. The API key is never logged. Video URLs are not logged.

```go
client, err := xai.NewClient(
    xai.WithLogLevel(xai.LevelInfo),
    xai.WithLogger(slog.Default()),
    xai.WithLogBodies(), // truncated protobuf JSON, debug only
)
```

| Level | What you get |
| --- | --- |
| `LevelDebug` | RPC started, stream flags, tool arguments, video poll progress |
| `LevelInfo` | RPC finished, uploads, video ready, agent run and tool calls |
| `LevelWarn` | canceled or deadline-exceeded calls, expired video, max turns |
| `LevelError` | failed RPCs and failed generations |

Pass your own handler with `WithLogger`. `WithLogLevel` still filters that logger.

## Agent

```go
runner := client.Runner()
session := agent.NewSession()
for ev, err := range runner.Run(ctx, session, assistant, "What time is it?") {
    if err != nil { return err }
    if ev.Kind == agent.KindText {
        fmt.Println(ev.Text)
    }
}
```

An agent has an instruction, a model, and client-side tools. The runner calls the model, executes function calls, and appends the results. Sub-agents are reached with the `handoff` tool. `RunSequential`, `RunParallel`, and `RunLoop` compose agents. Hooks can reject a model call or a tool call.

Server-side tools (`chat.WebSearch`, `chat.CodeExecution`, and the others) stay on the request and run on the xAI side.

The runner uses one completion per turn. A handoff lasts for that `Run` only. `RunParallel` yields events from cloned sessions and does not write those transcripts back. `Chat.Defer` returns a request id. Video generation polls once a second for ten minutes.

## License

Apache-2.0. Pull requests, tests, and version tags are in [CONTRIBUTING.md](CONTRIBUTING.md).
