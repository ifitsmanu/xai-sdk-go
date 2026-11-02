package agent_test

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ifitsmanu/xai-sdk-go/agent"
	"github.com/ifitsmanu/xai-sdk-go/responses"
	"github.com/ifitsmanu/xai-sdk-go/tool"
)

// fake streams scripted responses and records each request.
type fake struct {
	mu    sync.Mutex
	reqs  []responses.Request
	reply func(n int, req responses.Request) *responses.Response
}

func (f *fake) Stream(_ context.Context, req responses.Request) iter.Seq2[*responses.Event, error] {
	f.mu.Lock()
	n := len(f.reqs)
	f.reqs = append(f.reqs, req)
	f.mu.Unlock()
	resp := f.reply(n, req)
	return func(yield func(*responses.Event, error) bool) {
		if t := resp.Text(); t != "" && !yield(&responses.Event{Type: "response.output_text.delta", Delta: t}, nil) {
			return
		}
		yield(&responses.Event{Type: "response.completed", Response: resp}, nil)
	}
}

func answer(text string, usd float64) *responses.Response {
	return &responses.Response{
		Output: []responses.Item{&responses.Message{Role: "assistant", Content: []responses.Content{{Type: "output_text", Text: text}}}},
		Usage:  &responses.Usage{TotalTokens: 10, CostInUSDTicks: int64(usd * responses.TicksPerUSD)},
	}
}

func calls(names ...string) *responses.Response {
	r := &responses.Response{Usage: &responses.Usage{TotalTokens: 10}}
	for i, n := range names {
		r.Output = append(r.Output, &responses.FunctionCall{CallID: n + string(rune('0'+i)), Name: n, Arguments: `{}`})
	}
	return r
}

func run(t *testing.T, r *agent.Runner, s *agent.Session, ag *agent.Agent, input string) ([]agent.Event, error) {
	t.Helper()
	var events []agent.Event
	for e, err := range r.Run(context.Background(), s, ag, input) {
		if err != nil {
			return events, err
		}
		events = append(events, e)
	}
	return events, nil
}

func fn(name string, run func() (string, error)) *tool.Function {
	return &tool.Function{Name: name, Run: func(context.Context, json.RawMessage) (string, error) { return run() }}
}

// The function result goes back to the model in the next request, and the
// session id is sent as the prompt cache key so turns reuse the cache.
func TestToolResultReturnsToModel(t *testing.T) {
	m := &fake{reply: func(n int, req responses.Request) *responses.Response {
		if n == 0 {
			return calls("lookup")
		}
		return answer("done", 0)
	}}
	s := agent.NewSession()
	ag := &agent.Agent{Name: "a", Model: "m", Tools: []tool.Tool{fn("lookup", func() (string, error) { return "42", nil }), tool.WebSearch{}}}
	events, err := run(t, agent.NewRunner(m), s, ag, "hi")
	if err != nil {
		t.Fatal(err)
	}
	second := m.reqs[1].Input.(responses.Items)
	out, ok := second[len(second)-1].(*responses.FunctionOutput)
	if !ok || out.Output != "42" || out.CallID != "lookup0" {
		t.Fatalf("last input item = %#v", second[len(second)-1])
	}
	if m.reqs[0].PromptCacheKey != s.ID || len(m.reqs[0].Tools) != 2 {
		t.Fatalf("request = %+v", m.reqs[0])
	}
	if last := events[len(events)-1]; last.Kind != agent.KindMessage || last.Text != "done" {
		t.Fatalf("last event = %+v", last)
	}
}

// Calls from one turn run at the same time, and their results keep call order.
func TestParallelToolsKeepOrder(t *testing.T) {
	var started atomic.Int32
	both := make(chan struct{})
	slow := func(result string) func() (string, error) {
		return func() (string, error) {
			if started.Add(1) == 2 {
				close(both)
			}
			select {
			case <-both:
				return result, nil
			case <-time.After(2 * time.Second):
				return "", errors.New("calls did not overlap")
			}
		}
	}
	m := &fake{reply: func(n int, _ responses.Request) *responses.Response {
		if n == 0 {
			return calls("first", "second")
		}
		return answer("ok", 0)
	}}
	ag := &agent.Agent{Name: "a", Model: "m", Tools: []tool.Tool{fn("first", slow("1")), fn("second", slow("2"))}}
	events, err := run(t, agent.NewRunner(m), agent.NewSession(), ag, "go")
	if err != nil {
		t.Fatal(err)
	}
	var results []string
	for _, e := range events {
		if e.Kind == agent.KindToolResult {
			results = append(results, e.Result)
		}
	}
	if strings.Join(results, ",") != "1,2" {
		t.Fatalf("results = %v", results)
	}
}

// A tool marked Writes never runs without a yes from the confirmer.
func TestWritesNeedConfirmation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		opts    []agent.Option
		want    string
		mustRun bool
	}{
		{"no confirmer", nil, "not run: this action needs confirmation and no confirmer is set", false},
		{"declined", []agent.Option{agent.WithConfirm(func(context.Context, agent.Call) (bool, error) { return false, nil })}, "not run: the user declined", false},
		{"approved", []agent.Option{agent.WithConfirm(func(context.Context, agent.Call) (bool, error) { return true, nil })}, "posted", true},
	} {
		ran := false
		post := &tool.Function{Name: "post", Writes: true, Run: func(context.Context, json.RawMessage) (string, error) { ran = true; return "posted", nil }}
		m := &fake{reply: func(n int, _ responses.Request) *responses.Response {
			if n == 0 {
				return calls("post")
			}
			return answer("ok", 0)
		}}
		events, err := run(t, agent.NewRunner(m, tc.opts...), agent.NewSession(), &agent.Agent{Name: "a", Model: "m", Tools: []tool.Tool{post}}, "post it")
		if err != nil {
			t.Fatal(err)
		}
		var got string
		for _, e := range events {
			if e.Kind == agent.KindToolResult {
				got = e.Result
			}
		}
		if got != tc.want || ran != tc.mustRun {
			t.Errorf("%s: result %q ran %v", tc.name, got, ran)
		}
	}
}

func TestHandoffSwitchesAgent(t *testing.T) {
	m := &fake{reply: func(n int, req responses.Request) *responses.Response {
		if n == 0 {
			r := calls(agent.HandoffTool)
			r.Output[0].(*responses.FunctionCall).Arguments = `{"agent":"billing"}`
			return r
		}
		return answer("billing here", 0)
	}}
	billing := &agent.Agent{Name: "billing", Description: "Invoices", Model: "m2", Instruction: "You handle invoices."}
	front := &agent.Agent{Name: "front", Model: "m1", SubAgents: []*agent.Agent{billing}}
	events, err := run(t, agent.NewRunner(m), agent.NewSession(), front, "my invoice")
	if err != nil {
		t.Fatal(err)
	}
	if m.reqs[1].Model != "m2" || m.reqs[1].Instructions != "You handle invoices." {
		t.Fatalf("second request = %+v", m.reqs[1])
	}
	if last := events[len(events)-1]; last.Agent != "billing" {
		t.Fatalf("last event agent = %s", last.Agent)
	}
}

// The cost limit uses the charge the server reports and stops before the next turn.
func TestCostLimitStopsRun(t *testing.T) {
	m := &fake{reply: func(int, responses.Request) *responses.Response {
		r := calls("again")
		r.Usage.CostInUSDTicks = responses.TicksPerUSD / 100 // one cent a turn
		return r
	}}
	ag := &agent.Agent{Name: "a", Model: "m", Tools: []tool.Tool{fn("again", func() (string, error) { return "", nil })}}
	_, err := run(t, agent.NewRunner(m, agent.WithLimits(agent.Limits{CostUSD: 0.025})), agent.NewSession(), ag, "loop")
	if !errors.Is(err, agent.ErrBudget) || len(m.reqs) != 3 {
		t.Fatalf("err = %v after %d turns, want ErrBudget after 3", err, len(m.reqs))
	}
}

// A child agent used as a tool answers on its own session; the parent's session only gets the result.
func TestAgentAsTool(t *testing.T) {
	m := &fake{reply: func(n int, req responses.Request) *responses.Response {
		switch req.Model {
		case "child":
			return answer("child answer", 0)
		}
		if n == 0 {
			r := calls("researcher")
			r.Output[0].(*responses.FunctionCall).Arguments = `{"task":"find it"}`
			return r
		}
		return answer("parent done", 0)
	}}
	r := agent.NewRunner(m)
	child := &agent.Agent{Name: "researcher", Description: "Finds things", Model: "child"}
	parent := &agent.Agent{Name: "lead", Model: "parent", Tools: []tool.Tool{r.AsTool(child)}}
	s := agent.NewSession()
	events, err := run(t, r, s, parent, "go")
	if err != nil {
		t.Fatal(err)
	}
	var result string
	for _, e := range events {
		if e.Kind == agent.KindToolResult {
			result = e.Result
		}
	}
	if result != "child answer" || len(s.Items) != 4 { // user, call, result, answer: no child items
		t.Fatalf("result %q, parent items %d", result, len(s.Items))
	}
}

// Structured output sends the schema; OutputKey saves the answer for the next
// agent's {key} placeholder.
func TestOutputKeyFeedsNextInstruction(t *testing.T) {
	m := &fake{reply: func(n int, _ responses.Request) *responses.Response {
		if n == 0 {
			return answer(`{"city":"Paris"}`, 0)
		}
		return answer("ok", 0)
	}}
	schema := map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}}
	extract := &agent.Agent{Name: "extract", Model: "m", Output: schema, OutputKey: "place"}
	plan := &agent.Agent{Name: "plan", Model: "m", Instruction: "Plan a trip to {place}. Keep {unknown} as is."}
	for _, err := range agent.NewRunner(m).RunSequential(context.Background(), agent.NewSession(), []*agent.Agent{extract, plan}, "I want to see the Louvre") {
		if err != nil {
			t.Fatal(err)
		}
	}
	if m.reqs[0].Text == nil || m.reqs[0].Text.Format.Type != "json_schema" {
		t.Fatalf("schema not sent: %+v", m.reqs[0].Text)
	}
	if got := m.reqs[1].Instructions; got != `Plan a trip to {"city":"Paris"}. Keep {unknown} as is.` {
		t.Fatalf("instruction = %q", got)
	}
}
