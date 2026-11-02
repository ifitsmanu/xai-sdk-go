//go:build live

// Package live calls the real API. Run with: make live (needs XAI_API_KEY).
// Model ids come from the API. Voice models are not listed by the API, so the
// realtime and transcription checks run only when these are set:
//
//	XAI_LIVE_VOICE_MODEL  realtime model, such as the docs' grok-voice-latest
//	XAI_LIVE_STT_MODEL    transcription model from the docs
package live

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ifitsmanu/xai-sdk-go"
	"github.com/ifitsmanu/xai-sdk-go/agent"
	"github.com/ifitsmanu/xai-sdk-go/models"
	"github.com/ifitsmanu/xai-sdk-go/responses"
	"github.com/ifitsmanu/xai-sdk-go/tool"
	"github.com/ifitsmanu/xai-sdk-go/voice"
)

func client(t *testing.T) (*xai.Client, context.Context) {
	t.Helper()
	if os.Getenv("XAI_API_KEY") == "" {
		t.Skip("XAI_API_KEY is not set")
	}
	c, err := xai.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	t.Cleanup(cancel)
	return c, ctx
}

func languageModel(t *testing.T, c *xai.Client, ctx context.Context) string {
	t.Helper()
	id, err := c.Models.Select(ctx, models.Need{Rank: models.Cheapest})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestAccountAndModels(t *testing.T) {
	c, ctx := client(t)
	me, err := c.Account.Me(ctx)
	if err != nil || me.TeamID == "" {
		t.Fatalf("me = %+v, err = %v", me, err)
	}
	all, err := c.Models.All(ctx)
	if err != nil || len(all) == 0 {
		t.Fatalf("models: %v", err)
	}
	for _, m := range all {
		if m.ContextLength > 0 && m.PromptTextTokenPrice == 0 && m.ImagePrice == 0 && len(m.Pricing) == 0 {
			t.Errorf("%s decoded with no price: the model struct is missing fields", m.ID)
		}
	}
	if _, err := c.Models.VideoNames(ctx); err != nil {
		t.Fatal(err)
	}
	if id, err := c.Models.Select(ctx, models.Need{}); err != nil || id == "" {
		t.Fatalf("select = %q, %v", id, err)
	} else {
		t.Logf("newest language model: %s", id)
	}
}

func TestResponsesCreateAndStream(t *testing.T) {
	c, ctx := client(t)
	store := false
	req := responses.Request{Model: languageModel(t, c, ctx), Input: responses.Text("Reply with one word: hello"), Store: &store}
	resp, err := c.Responses.Create(ctx, req)
	if err != nil || resp.Text() == "" || resp.Usage == nil || resp.Usage.TotalTokens == 0 {
		t.Fatalf("create = %+v, err = %v", resp, err)
	}
	t.Logf("cost $%.6f", resp.Usage.USD())
	var text strings.Builder
	var final *responses.Response
	for event, err := range c.Responses.Stream(ctx, req) {
		if err != nil {
			t.Fatal(err)
		}
		switch event.Type {
		case "response.output_text.delta":
			text.WriteString(event.Delta)
		case "response.completed":
			final = event.Response
		}
	}
	if text.Len() == 0 || final == nil || final.Text() != text.String() {
		t.Fatalf("streamed %q, final %+v", text.String(), final)
	}
}

// The model calls a client function, gets the result, and answers with it.
func TestResponsesFunctionCallTurn(t *testing.T) {
	c, ctx := client(t)
	req := responses.Request{
		Model: languageModel(t, c, ctx),
		Input: responses.Items{responses.UserText("What is the secret code? Use the tool.")},
		Tools: []responses.Tool{responses.Function{
			Name:        "secret_code",
			Description: "Returns the secret code.",
			Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		}},
		ToolChoice: &responses.ToolChoice{Function: "secret_code"},
	}
	first, err := c.Responses.Create(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	calls := first.FunctionCalls()
	if len(calls) != 1 || calls[0].Name != "secret_code" {
		t.Fatalf("calls = %+v", first.Output)
	}
	items := append(req.Input.(responses.Items), first.Output...)
	items = append(items, &responses.FunctionOutput{CallID: calls[0].CallID, Output: "PINEAPPLE-42"})
	req.Input, req.ToolChoice = items, nil
	second, err := c.Responses.Create(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(second.Text(), "PINEAPPLE-42") {
		t.Fatalf("answer %q does not use the tool result", second.Text())
	}
}

// Server-side web search with domain filters, the shape the API reference documents.
func TestResponsesWebSearch(t *testing.T) {
	c, ctx := client(t)
	resp, err := c.Responses.Create(ctx, responses.Request{
		Model: languageModel(t, c, ctx),
		Input: responses.Text("Search the web: what is the title of the page at docs.x.ai? Answer in a few words."),
		Tools: []responses.Tool{responses.WebSearch{Filters: &responses.SearchFilters{AllowedDomains: []string{"docs.x.ai"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	searched := false
	for _, item := range resp.Output {
		if o, ok := item.(*responses.OtherItem); ok && o.Type == "web_search_call" {
			searched = true
		}
	}
	if !searched || resp.Text() == "" {
		t.Fatalf("searched=%v text=%q output=%d items", searched, resp.Text(), len(resp.Output))
	}
	t.Logf("server tools used %d, cost $%.6f", resp.Usage.NumServerSideToolsUsed, resp.Usage.USD())
}

func TestSpeakAndTranscribe(t *testing.T) {
	c, ctx := client(t)
	audio, err := c.Voice.Speak(ctx, voice.Speech{Text: "Hello there, friend.", Language: "en"})
	if err != nil || len(audio.Data) == 0 || !strings.HasPrefix(audio.ContentType, "audio/") {
		t.Fatalf("speak: %d bytes, %q, %v", len(audio.Data), audio.ContentType, err)
	}
	model := os.Getenv("XAI_LIVE_STT_MODEL")
	if model == "" {
		t.Skip("XAI_LIVE_STT_MODEL is not set")
	}
	got, err := c.Voice.Transcribe(ctx, voice.Transcription{Model: model, Audio: audio.Data, Filename: "hello.mp3", Language: "en"})
	if err != nil || !strings.Contains(strings.ToLower(got.Text), "hello") {
		t.Fatalf("transcript = %+v, err = %v", got, err)
	}
}

// A spoken turn where the model calls a client function mid-conversation and says
// the result. Audio deltas exceed 100 KB, which failed before the read limit was raised.
func TestRealtimeVoiceAgentToolCall(t *testing.T) {
	c, ctx := client(t)
	model := os.Getenv("XAI_LIVE_VOICE_MODEL")
	if model == "" {
		t.Skip("XAI_LIVE_VOICE_MODEL is not set")
	}
	sock, err := c.Voice.Dial(ctx, url.Values{"model": {model}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sock.Close() }()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(sock.Update(ctx, voice.Session{
		Instructions: "When asked for the secret code, call secret_code and read the result aloud.",
		Tools: []voice.Tool{voice.Function{
			Name: "secret_code", Description: "Returns the secret code.",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{}},
		}},
	}))
	must(sock.SendText(ctx, "user", "What is the secret code?"))
	must(sock.Respond(ctx))

	var transcript strings.Builder
	callResponse, largest, audio := "", 0, 0
	for {
		e, err := sock.Next(ctx)
		if err != nil {
			t.Fatalf("after %d audio bytes: %v", audio, err)
		}
		switch e.Type {
		case voice.EventError:
			t.Fatalf("server error: %+v", e.Error)
		case voice.EventFunctionCall:
			callResponse = e.ResponseID
			must(sock.SendFunctionOutput(ctx, e.CallID, "PINEAPPLE-42"))
			must(sock.Respond(ctx))
		case voice.EventAudioDelta:
			pcm, err := e.Audio()
			must(err)
			audio += len(pcm)
			largest = max(largest, len(e.Delta))
		case voice.EventTranscriptDelta:
			if callResponse != "" && e.ResponseID != callResponse {
				transcript.WriteString(e.Delta)
			}
		}
		if e.Type == voice.EventResponseDone && callResponse != "" && e.Response != nil && e.Response.ID != callResponse {
			break
		}
	}
	t.Logf("said %q; %d audio bytes, largest delta %d bytes", transcript.String(), audio, largest)
	if !strings.Contains(strings.ToLower(transcript.String()), "pineapple") {
		t.Fatal("the spoken answer does not use the tool result")
	}
}

// A lead agent delegates to a researcher agent used as a tool, under a cost cap.
func TestAgentDelegatesWithinBudget(t *testing.T) {
	c, ctx := client(t)
	model := languageModel(t, c, ctx)
	runner := c.Runner(agent.WithLimits(agent.Limits{CostUSD: 0.05, Turns: 6}))
	code, err := tool.Func("vault_code", "Reads the vault code.", func(context.Context, struct{}) (string, error) { return "ORCHID-7", nil })
	if err != nil {
		t.Fatal(err)
	}
	researcher := &agent.Agent{Name: "researcher", Description: "Looks things up with its tools.", Model: model,
		Instruction: "Use vault_code to answer. Reply with the code only.", Tools: []tool.Tool{code}}
	lead := &agent.Agent{Name: "lead", Model: model,
		Instruction: "Delegate lookups to researcher, then answer in one sentence.", Tools: []tool.Tool{runner.AsTool(researcher)}}
	var answer string
	var spent float64
	for e, err := range runner.Run(ctx, agent.NewSession(), lead, "What is the vault code?") {
		if err != nil {
			t.Fatal(err)
		}
		switch e.Kind {
		case agent.KindMessage:
			answer = e.Text
		case agent.KindTurn:
			spent += e.Usage.USD()
		}
	}
	t.Logf("answer %q, lead cost $%.4f", answer, spent)
	if !strings.Contains(answer, "ORCHID-7") {
		t.Fatal("the lead did not use the researcher's answer")
	}
}
