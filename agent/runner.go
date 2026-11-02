package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"sync"
	"time"

	"github.com/ifitsmanu/xai-sdk-go/responses"
	"github.com/ifitsmanu/xai-sdk-go/tool"
)

// Errors that end a run because a limit was reached.
var (
	ErrMaxTurns = errors.New("agent: turn limit reached")
	ErrBudget   = errors.New("agent: token or cost limit reached")
)

// Model streams one Responses turn. *responses.Service implements it.
type Model interface {
	Stream(ctx context.Context, req responses.Request) iter.Seq2[*responses.Event, error]
}

// Kind classifies a run event.
type Kind int

const (
	KindDelta      Kind = iota // a piece of streamed answer text
	KindMessage                // the full answer text of one model turn
	KindTurn                   // a model turn finished; Usage and Response are set
	KindToolCall               // a function the runner is about to run
	KindToolResult             // the function's result
	KindHandoff                // the active agent changed; Text is the new agent
)

// Event is one step of a run.
type Event struct {
	Agent    string
	Kind     Kind
	Text     string
	Tool     string
	Args     string
	Result   string
	Usage    *responses.Usage
	Response *responses.Response
	Latency  time.Duration
}

// Limits bound a whole run, including handoffs.
// Zero values do not limit, except Turns, which defaults to DefaultTurns.
type Limits struct {
	Turns   int
	Tokens  int64
	CostUSD float64
	Time    time.Duration
}

// DefaultTurns is the model-turn limit when Limits.Turns is zero.
const DefaultTurns = 16

// Confirm decides whether a tool marked Writes may run.
type Confirm func(ctx context.Context, call Call) (bool, error)

// Runner executes agents.
type Runner struct {
	model       Model
	store       Store
	logger      *slog.Logger
	limits      Limits
	confirm     Confirm
	concurrency int
}

// Option configures a Runner.
type Option func(*Runner)

// WithLogger logs runs, turns, and tools. Debug includes tool arguments.
func WithLogger(l *slog.Logger) Option {
	return func(r *Runner) { r.logger = l.With("subsystem", "agent") }
}

// WithStore saves the session after each run.
func WithStore(s Store) Option { return func(r *Runner) { r.store = s } }

// WithLimits bounds every run.
func WithLimits(l Limits) Option { return func(r *Runner) { r.limits = l } }

// WithConfirm asks before running tools marked Writes. Without it they are refused.
func WithConfirm(c Confirm) Option { return func(r *Runner) { r.confirm = c } }

// WithToolConcurrency runs up to n function calls from one turn at once.
// The default is 8; 1 runs them in order.
func WithToolConcurrency(n int) Option { return func(r *Runner) { r.concurrency = n } }

// NewRunner runs agents on model, usually client.Responses.
func NewRunner(model Model, opts ...Option) *Runner {
	r := &Runner{model: model, logger: slog.New(slog.DiscardHandler), concurrency: 8}
	for _, opt := range opts {
		opt(r)
	}
	if r.limits.Turns <= 0 {
		r.limits.Turns = DefaultTurns
	}
	return r
}

// Run adds input as a user message when non-empty, then alternates model turns
// and function calls until the model answers without calling a function.
func (r *Runner) Run(ctx context.Context, s *Session, ag *Agent, input string) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		if r.limits.Time > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, r.limits.Time)
			defer cancel()
		}
		b := &budget{limits: r.limits}
		if err := r.run(context.WithValue(ctx, sessionKey{}, s), s, ag, input, b, yield); err != nil {
			yield(Event{Agent: ag.Name}, err)
		}
	}
}

type budget struct {
	mu     sync.Mutex
	limits Limits
	turns  int
	tokens int64
	usd    float64
}

func (b *budget) startTurn() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.turns >= b.limits.Turns {
		return ErrMaxTurns
	}
	if (b.limits.Tokens > 0 && b.tokens >= b.limits.Tokens) || (b.limits.CostUSD > 0 && b.usd >= b.limits.CostUSD) {
		return ErrBudget
	}
	b.turns++
	return nil
}

func (b *budget) spend(u *responses.Usage) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if u != nil {
		b.tokens += u.TotalTokens
		b.usd += u.USD()
	}
}

func (r *Runner) run(ctx context.Context, s *Session, ag *Agent, input string, b *budget, yield func(Event, error) bool) error {
	if input != "" {
		s.Items = append(s.Items, responses.UserText(input))
	}
	r.logger.InfoContext(ctx, "agent run", "agent", ag.Name, "session", s.ID)
	for {
		if err := b.startTurn(); err != nil {
			return errors.Join(err, r.save(ctx, s))
		}
		resp, ok, err := r.turn(ctx, s, ag, b, yield)
		if err != nil || !ok {
			return err
		}
		calls := resp.FunctionCalls()
		if len(calls) == 0 {
			if ag.OutputKey != "" {
				s.Set(ag.OutputKey, resp.Text())
			}
			return r.save(ctx, s)
		}
		next, ok, err := r.callTools(ctx, s, ag, calls, yield)
		if err != nil || !ok {
			return err
		}
		ag = next
	}
}

// turn streams one model turn and appends its output to the session.
func (r *Runner) turn(ctx context.Context, s *Session, ag *Agent, b *budget, yield func(Event, error) bool) (*responses.Response, bool, error) {
	req, err := ag.request(s)
	if err != nil {
		return nil, false, err
	}
	if h := ag.Hooks.BeforeModel; h != nil {
		if err := h(ctx, ag.Name, &req); err != nil {
			return nil, false, err
		}
	}
	start := time.Now()
	var resp *responses.Response
	for e, err := range r.model.Stream(ctx, req) {
		if err != nil {
			return nil, false, err
		}
		switch e.Type {
		case "response.output_text.delta":
			if !yield(Event{Agent: ag.Name, Kind: KindDelta, Text: e.Delta}, nil) {
				return nil, false, nil
			}
		case "response.completed", "response.incomplete", "response.failed":
			resp = e.Response
		}
	}
	if resp == nil {
		return nil, false, errors.New("agent: stream ended without a response")
	}
	if resp.Error != nil {
		return nil, false, fmt.Errorf("agent %s: %s: %s", ag.Name, resp.Error.Code, resp.Error.Message)
	}
	latency := time.Since(start)
	b.spend(resp.Usage)
	s.Items = append(s.Items, resp.Output...)
	r.logger.InfoContext(ctx, "agent turn", "agent", ag.Name, "duration", latency, "usd", resp.Usage.USD())
	if h := ag.Hooks.AfterModel; h != nil {
		if err := h(ctx, ag.Name, resp); err != nil {
			return nil, false, err
		}
	}
	if !yield(Event{Agent: ag.Name, Kind: KindTurn, Usage: resp.Usage, Response: resp, Latency: latency}, nil) {
		return nil, false, nil
	}
	if text := resp.Text(); text != "" && !yield(Event{Agent: ag.Name, Kind: KindMessage, Text: text, Response: resp}, nil) {
		return nil, false, nil
	}
	return resp, true, nil
}

type outcome struct {
	result  string
	err     error
	latency time.Duration
}

// callTools runs one turn's function calls, at most r.concurrency at a time,
// and records the results in call order. It returns the agent for the next turn.
func (r *Runner) callTools(ctx context.Context, s *Session, ag *Agent, calls []*responses.FunctionCall, yield func(Event, error) bool) (*Agent, bool, error) {
	for _, c := range calls {
		if !yield(Event{Agent: ag.Name, Kind: KindToolCall, Tool: c.Name, Args: c.Arguments}, nil) {
			return nil, false, nil
		}
	}
	fns := tool.Functions(ag.Tools)
	results := make([]outcome, len(calls))
	next := ag
	slots := make(chan struct{}, max(r.concurrency, 1))
	var wg sync.WaitGroup
	for i, c := range calls {
		if c.Name == HandoffTool {
			results[i].result, next = handoff(ag, c.Arguments)
			continue
		}
		call := Call{ID: c.CallID, Name: c.Name, Args: json.RawMessage(c.Arguments)}
		if h := ag.Hooks.BeforeTool; h != nil {
			if err := h(ctx, call); err != nil {
				wg.Wait()
				return nil, false, err
			}
		}
		wg.Add(1)
		slots <- struct{}{}
		go func() {
			defer func() { <-slots; wg.Done() }()
			start := time.Now()
			result, err := r.execute(ctx, fns[c.Name], call)
			results[i] = outcome{result, err, time.Since(start)}
			if h := ag.Hooks.AfterTool; h != nil {
				h(ctx, call, result, err)
			}
		}()
	}
	wg.Wait()
	for i, c := range calls {
		res := results[i]
		if res.err != nil {
			res.result = "error: " + res.err.Error()
		}
		s.Items = append(s.Items, &responses.FunctionOutput{CallID: c.CallID, Output: res.result})
		r.logger.DebugContext(ctx, "agent tool", "tool", c.Name, "args", c.Arguments, "duration", res.latency, "err", res.err)
		if !yield(Event{Agent: ag.Name, Kind: KindToolResult, Tool: c.Name, Args: c.Arguments, Result: res.result, Latency: res.latency}, nil) {
			return nil, false, nil
		}
	}
	if next != ag && !yield(Event{Agent: next.Name, Kind: KindHandoff, Text: next.Name}, nil) {
		return nil, false, nil
	}
	return next, true, nil
}

func (r *Runner) execute(ctx context.Context, fn *tool.Function, call Call) (string, error) {
	if fn == nil || fn.Run == nil {
		return "", fmt.Errorf("unknown tool %s", call.Name)
	}
	if fn.Writes {
		if r.confirm == nil {
			return "not run: this action needs confirmation and no confirmer is set", nil
		}
		ok, err := r.confirm(ctx, call)
		if err != nil {
			return "", err
		}
		if !ok {
			return "not run: the user declined", nil
		}
	}
	return fn.Run(ctx, call.Args)
}

func handoff(from *Agent, args string) (string, *Agent) {
	var in struct {
		Agent string `json:"agent"`
	}
	_ = json.Unmarshal([]byte(args), &in)
	if to := from.findSub(in.Agent); to != nil {
		return "handed off to " + to.Name, to
	}
	return "error: no agent named " + in.Agent, from
}

func (r *Runner) save(ctx context.Context, s *Session) error {
	if r.store == nil {
		return nil
	}
	return r.store.Save(ctx, s)
}

// AsTool lets another agent delegate a task to ag. The child runs on a fresh
// session with this runner's limits and returns its final answer.
func (r *Runner) AsTool(ag *Agent) *tool.Function {
	return &tool.Function{
		Name:        ag.Name,
		Description: ag.Description,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{"task": map[string]any{"type": "string", "description": "What to do, with all needed context."}},
			"required":   []string{"task"},
		},
		Run: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in struct {
				Task string `json:"task"`
			}
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", err
			}
			var answer string
			for e, err := range r.Run(ctx, NewSession(), ag, in.Task) {
				if err != nil {
					return "", err
				}
				if e.Kind == KindMessage {
					answer = e.Text
				}
			}
			return answer, nil
		},
	}
}

// RunSequential runs agents one after another on the same session.
// Only the first receives input; later agents see the earlier answers.
func (r *Runner) RunSequential(ctx context.Context, s *Session, agents []*Agent, input string) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		for i, ag := range agents {
			if i > 0 {
				input = ""
			}
			for e, err := range r.Run(ctx, s, ag, input) {
				if !yield(e, err) || err != nil {
					return
				}
			}
		}
	}
}

// RunParallel runs each agent on its own copy of the session and merges events as they arrive.
func (r *Runner) RunParallel(ctx context.Context, s *Session, agents []*Agent, input string) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		type item struct {
			e   Event
			err error
		}
		ch := make(chan item)
		var wg sync.WaitGroup
		branch := *r
		branch.store = nil
		for _, ag := range agents {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for e, err := range branch.Run(ctx, s.Clone(), ag, input) {
					select {
					case ch <- item{e, err}:
					case <-ctx.Done():
						return
					}
				}
			}()
		}
		go func() { wg.Wait(); close(ch) }()
		for it := range ch {
			if !yield(it.e, it.err) {
				return
			}
		}
	}
}

// RunLoop repeats ag until done reports true for an event or iterations run out.
// Give the agent the Done tool and pass IsDone to let the model end the loop.
func (r *Runner) RunLoop(ctx context.Context, s *Session, ag *Agent, input string, iterations int, done func(Event) bool) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		for range max(iterations, 1) {
			for e, err := range r.Run(ctx, s, ag, input) {
				if !yield(e, err) || err != nil || (done != nil && done(e)) {
					return
				}
			}
			input = ""
		}
	}
}
