package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"strings"
	"sync"
	"time"

	"log/slog"

	"github.com/ifitsmanu/xai-sdk-go/chat"
	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
)

// ErrMaxTurns is returned when the client-side tool loop hits its limit.
var ErrMaxTurns = errors.New("agent: max turns exceeded")

const stoppedResult = "agent: stopped"

// Kind classifies a run event.
type Kind int

const (
	// KindModel is a model response.
	KindModel Kind = iota
	// KindText is assistant text from that response.
	KindText
	// KindToolCall is a client-side tool the runner is about to execute.
	KindToolCall
	// KindToolResult is the tool's return value.
	KindToolResult
	// KindHandoff means the active agent changed.
	KindHandoff
)

// Event is one step of a run.
type Event struct {
	Agent string
	Kind  Kind
	Text  string
	Tool  string
	Args  string
	// Result is the tool return value.
	Result string
	// Response is the completion for KindModel and KindText.
	Response *chat.Response
	// Latency is how long this step took.
	// KindModel is the completion RPC. KindToolResult is the tool callback.
	Latency time.Duration
}

// Completer samples one chat completion. *chat.Service implements it.
type Completer interface {
	Complete(ctx context.Context, req *xaiapi.GetCompletionsRequest) (*xaiapi.GetChatCompletionResponse, error)
}

// Runner executes agents.
type Runner struct {
	complete Completer
	store    Store
	logger   *slog.Logger
}

// Option configures a Runner.
type Option func(*Runner)

// WithLogger sets the runner logger. Nil keeps logs discarded.
// Debug includes tool arguments. Info includes run, handoff, and tool names.
func WithLogger(logger *slog.Logger) Option {
	return func(r *Runner) {
		if logger != nil {
			r.logger = logger.With("subsystem", "agent")
		}
	}
}

// WithStore saves the session after a successful run.
func WithStore(store Store) Option {
	return func(r *Runner) { r.store = store }
}

// NewRunner samples with complete, which is usually client.Chat.
func NewRunner(complete Completer, opts ...Option) *Runner {
	r := &Runner{complete: complete, logger: discardLogger()}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.Level(1 << 30)}))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// Run appends input as a user message when it is non-empty, then loops
// model calls and client-side tools until the model stops calling them.
func (r *Runner) Run(ctx context.Context, session *Session, ag *Agent, input string) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		if err := r.run(ctx, session, ag, input, yield); err != nil {
			yield(Event{}, err)
		}
	}
}

func (r *Runner) run(ctx context.Context, session *Session, ag *Agent, input string, yield func(Event, error) bool) error {
	if ag == nil {
		return errors.New("agent: agent is nil")
	}
	if ag.Model == "" {
		return fmt.Errorf("agent: %s has no model", ag.Name)
	}
	if session == nil {
		return errors.New("agent: session is nil")
	}
	ctx = withSession(ctx, session)
	r.log(ctx, slog.LevelInfo, "agent run started", "agent", ag.Name, "session", session.ID, "model", ag.Model)
	if input != "" && !trailingUser(session, input) {
		session.Messages = append(session.Messages, chat.UserText(input))
	}

	current := ag
	turns := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if turns >= current.maxTurns() {
			r.log(ctx, slog.LevelWarn, "agent max turns", "agent", current.Name, "turns", turns)
			if err := r.save(ctx, session); err != nil {
				return err
			}
			return ErrMaxTurns
		}
		turns++

		specs, byName, err := current.toolSpecs()
		if err != nil {
			return err
		}
		setInstruction(session, current.systemPrompt())
		req := &xaiapi.GetCompletionsRequest{
			Model:    current.Model,
			Messages: session.Messages,
			Tools:    specs,
			MaxTurns: current.ServerMaxTurns,
		}
		if current.Hooks.BeforeModel != nil {
			if err := current.Hooks.BeforeModel(ctx, current.Name, req); err != nil {
				return err
			}
		}
		r.log(ctx, slog.LevelDebug, "agent model request", "agent", current.Name, "model", current.Model, "messages", len(session.Messages), "tools", len(specs))
		started := time.Now()
		raw, err := r.complete.Complete(ctx, req)
		latency := time.Since(started)
		if err != nil {
			r.log(ctx, slog.LevelError, "agent model call failed", "agent", current.Name, "error", err.Error(), "duration", latency)
			return err
		}
		r.log(ctx, slog.LevelInfo, "agent model finished", "agent", current.Name, "duration", latency)
		resp := &chat.Response{Proto: raw}
		if current.Hooks.AfterModel != nil {
			if err := current.Hooks.AfterModel(ctx, current.Name, resp); err != nil {
				return err
			}
		}
		session.Messages = append(session.Messages, chat.MessagesFromResponse(resp)...)
		text := resp.Content()
		if !yield(Event{Agent: current.Name, Kind: KindModel, Response: resp, Text: text, Latency: latency}, nil) {
			return nil
		}
		if text != "" {
			if !yield(Event{Agent: current.Name, Kind: KindText, Text: text, Response: resp}, nil) {
				return nil
			}
		}

		var calls []pendingCall
		for _, tc := range resp.ToolCalls() {
			fn, ok := chat.ClientFunction(tc)
			if !ok {
				continue
			}
			calls = append(calls, pendingCall{id: tc.GetId(), name: fn.GetName(), args: fn.GetArguments()})
		}
		if len(calls) == 0 {
			r.log(ctx, slog.LevelInfo, "agent run finished", "agent", current.Name, "session", session.ID)
			return r.save(ctx, session)
		}

		active := current
		handedOff := false
		budgetSpent := turns >= current.maxTurns()
		for i, call := range calls {
			if budgetSpent && call.name != HandoffTool {
				seal(session, calls[i:], "agent: max turns exceeded")
				if err := r.save(ctx, session); err != nil {
					return err
				}
				return ErrMaxTurns
			}
			ev := Event{Agent: active.Name, Kind: KindToolCall, Tool: call.name, Args: call.args}
			if !yield(ev, nil) {
				seal(session, calls[i:], stoppedResult)
				return nil
			}
			if call.name == HandoffTool {
				next, result, herr := applyHandoff(active, call.args)
				if herr != nil && result == "" {
					result = herr.Error()
				}
				session.Messages = append(session.Messages, chat.ToolResult(result, call.id))
				if herr != nil {
					if !yield(Event{Agent: active.Name, Kind: KindToolResult, Tool: call.name, Result: result}, nil) {
						seal(session, calls[i+1:], stoppedResult)
						return nil
					}
					continue
				}
				if !yield(Event{Agent: next.Name, Kind: KindHandoff, Tool: call.name, Text: next.Name, Result: result}, nil) {
					seal(session, calls[i+1:], stoppedResult)
					return nil
				}
				r.log(ctx, slog.LevelInfo, "agent handoff", "from", active.Name, "to", next.Name, "session", session.ID)
				current = next
				handedOff = true
				continue
			}
			tool := byName[call.name]
			r.log(ctx, slog.LevelDebug, "agent tool arguments", "agent", active.Name, "tool", call.name, "args", truncate(call.args, 512))
			started := time.Now()
			result, stop, err := r.execTool(ctx, active, call, tool)
			elapsed := time.Since(started)
			if stop {
				msg := "agent: blocked"
				if err != nil {
					msg = err.Error()
				}
				r.log(ctx, slog.LevelWarn, "agent tool rejected", "agent", active.Name, "tool", call.name, "error", msg, "duration", elapsed)
				seal(session, calls[i:], msg)
				return err
			}
			r.log(ctx, slog.LevelInfo, "agent tool finished", "agent", active.Name, "tool", call.name, "duration", elapsed, "error", errString(err))
			if err != nil && result == "" {
				result = err.Error()
			}
			session.Messages = append(session.Messages, chat.ToolResult(result, call.id))
			if !yield(Event{Agent: active.Name, Kind: KindToolResult, Tool: call.name, Args: call.args, Result: result, Latency: elapsed}, nil) {
				seal(session, calls[i+1:], stoppedResult)
				return nil
			}
		}
		if handedOff {
			turns = 0
		}
	}
}

func seal(session *Session, calls []pendingCall, result string) {
	for _, call := range calls {
		session.Messages = append(session.Messages, chat.ToolResult(result, call.id))
	}
}

func trailingUser(session *Session, input string) bool {
	if session == nil || len(session.Messages) == 0 {
		return false
	}
	last := session.Messages[len(session.Messages)-1]
	return last.GetRole() == xaiapi.MessageRole_ROLE_USER && len(last.GetContent()) == 1 && last.GetContent()[0].GetText() == input
}

type pendingCall struct {
	id   string
	name string
	args string
}

func (r *Runner) execTool(ctx context.Context, ag *Agent, call pendingCall, tool Tool) (string, bool, error) {
	c := Call{ID: call.id, Name: call.name, Args: json.RawMessage(call.args)}
	if ag.Hooks.BeforeTool != nil {
		if err := ag.Hooks.BeforeTool(ctx, c); err != nil {
			return "", true, err
		}
	}
	if tool == nil {
		err := fmt.Errorf("agent: unknown tool %s", call.name)
		if ag.Hooks.AfterTool != nil {
			ag.Hooks.AfterTool(ctx, c, "", err)
		}
		return "", false, err
	}
	result, err := tool.Run(ctx, c.Args)
	if ag.Hooks.AfterTool != nil {
		ag.Hooks.AfterTool(ctx, c, result, err)
	}
	return result, false, err
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func applyHandoff(current *Agent, args string) (*Agent, string, error) {
	var payload struct {
		Agent string `json:"agent"`
	}
	if err := json.Unmarshal([]byte(args), &payload); err != nil {
		return nil, "", err
	}
	next := current.findSub(payload.Agent)
	if next == nil {
		return nil, "", fmt.Errorf("agent: no agent named %s", payload.Agent)
	}
	return next, "Handed off to " + next.Name + ".", nil
}

func setInstruction(session *Session, instruction string) {
	if strings.TrimSpace(instruction) == "" {
		return
	}
	if len(session.Messages) > 0 && sameInstruction(session.Messages[0], instruction) {
		return
	}
	sys := chat.SystemText(instruction)
	if len(session.Messages) == 0 {
		session.Messages = []*xaiapi.Message{sys}
		return
	}
	if session.Messages[0] != nil && session.Messages[0].Role == xaiapi.MessageRole_ROLE_SYSTEM {
		session.Messages[0] = sys
		return
	}
	session.Messages = append([]*xaiapi.Message{sys}, session.Messages...)
}

func sameInstruction(msg *xaiapi.Message, instruction string) bool {
	if msg == nil || msg.Role != xaiapi.MessageRole_ROLE_SYSTEM || len(msg.Content) != 1 || msg.Content[0] == nil {
		return false
	}
	return msg.Content[0].GetText() == instruction
}

func (r *Runner) log(ctx context.Context, level slog.Level, msg string, args ...any) {
	if r.logger == nil || !r.logger.Enabled(ctx, level) {
		return
	}
	r.logger.Log(ctx, level, msg, args...)
}

func (r *Runner) save(ctx context.Context, session *Session) error {
	if r.store == nil {
		return nil
	}
	return r.store.Save(ctx, session)
}

// RunSequential runs each agent on the same transcript.
// Only the first agent receives input. Later agents see the earlier replies already stored.
func (r *Runner) RunSequential(ctx context.Context, session *Session, agents []*Agent, input string) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		next := input
		for i, ag := range agents {
			send := next
			if i > 0 {
				send = ""
			}
			for ev, err := range r.Run(ctx, session, ag, send) {
				if err != nil {
					yield(ev, err)
					return
				}
				if !yield(ev, nil) {
					return
				}
			}
			next = ""
		}
	}
}

// RunParallel runs each agent on a clone of the session and merges events as they arrive.
func (r *Runner) RunParallel(ctx context.Context, session *Session, agents []*Agent, input string) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		type item struct {
			ev  Event
			err error
		}
		ch := make(chan item)
		var wg sync.WaitGroup
		for _, ag := range agents {
			wg.Add(1)
			go func(ag *Agent) {
				defer wg.Done()
				branch := *r
				branch.store = nil
				sess := session.Clone()
				for ev, err := range branch.Run(ctx, sess, ag, input) {
					select {
					case <-ctx.Done():
						return
					case ch <- item{ev, err}:
					}
					if err != nil {
						return
					}
				}
			}(ag)
		}
		go func() {
			wg.Wait()
			close(ch)
		}()
		for it := range ch {
			if !yield(it.ev, it.err) {
				return
			}
		}
	}
}

// RunLoop repeats the agent until stop is set, until returns true, or max iterations elapse.
// A nil until only watches stop and the turn limit inside each Run.
func (r *Runner) RunLoop(ctx context.Context, session *Session, ag *Agent, input string, maxIterations int, stop *bool, until func(Event) bool) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		if maxIterations <= 0 {
			maxIterations = 1
		}
		next := input
		for i := 0; i < maxIterations; i++ {
			if stop != nil && *stop {
				return
			}
			for ev, err := range r.Run(ctx, session, ag, next) {
				if err != nil {
					yield(ev, err)
					return
				}
				if !yield(ev, nil) {
					return
				}
				if until != nil && until(ev) {
					return
				}
				if stop != nil && *stop {
					return
				}
			}
			next = ""
		}
	}
}
