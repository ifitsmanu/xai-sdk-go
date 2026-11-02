package responses_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ifitsmanu/xai-sdk-go/internal/rpc"
	"github.com/ifitsmanu/xai-sdk-go/responses"
)

func service(t *testing.T, h http.HandlerFunc) *responses.Service {
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return responses.NewService(&rpc.Conn{RESTBase: srv.URL, APIKey: "k", Logger: rpc.NopLogger()})
}

// The request uses the wire shapes from the API reference, and the response keeps
// unknown server items intact so they can be sent back as input.
func TestToolTurnRoundTrip(t *testing.T) {
	var sent map[string]any
	svc := service(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &sent)
		_, _ = w.Write([]byte(`{"id":"r1","status":"completed","output":[
			{"type":"web_search_call","id":"ws1","status":"completed","action":{"query":"paris"}},
			{"type":"function_call","id":"fc1","call_id":"c1","name":"weather","arguments":"{\"city\":\"Paris\"}"},
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Checking."}]}],
			"usage":{"input_tokens":10,"output_tokens":3,"total_tokens":13,"cost_in_usd_ticks":4006000}}`))
	})
	resp, err := svc.Create(context.Background(), responses.Request{
		Model: "m",
		Input: responses.Items{responses.UserText("Weather in Paris?")},
		Tools: []responses.Tool{
			responses.Function{Name: "weather", Parameters: map[string]any{"type": "object"}},
			responses.WebSearch{Filters: &responses.SearchFilters{AllowedDomains: []string{"meteo.fr"}}},
		},
		ToolChoice: &responses.ToolChoice{Mode: "auto"},
	})
	if err != nil {
		t.Fatal(err)
	}

	tools := sent["tools"].([]any)
	if tools[0].(map[string]any)["type"] != "function" || tools[1].(map[string]any)["type"] != "web_search" {
		t.Fatalf("tools = %v", tools)
	}
	if sent["tool_choice"] != "auto" || sent["input"].([]any)[0].(map[string]any)["type"] != "message" {
		t.Fatalf("request = %v", sent)
	}

	calls := resp.FunctionCalls()
	if len(calls) != 1 || calls[0].CallID != "c1" || resp.Text() != "Checking." || resp.Usage.USD() != 0.0004006 {
		t.Fatalf("response = %+v", resp)
	}
	other, ok := resp.Output[0].(*responses.OtherItem)
	if !ok || other.Type != "web_search_call" {
		t.Fatalf("output[0] = %#v", resp.Output[0])
	}
	again, _ := json.Marshal(other)
	if !strings.Contains(string(again), `"query":"paris"`) {
		t.Fatalf("server item lost fields when sent back: %s", again)
	}
}

func TestStreamEvents(t *testing.T) {
	svc := service(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["stream"] != true {
			t.Errorf("stream flag = %v", body["stream"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: response.output_text.delta\n"+
			`data: {"type":"response.output_text.delta","delta":"Hel"}`+"\n\n"+
			`data: {"type":"response.output_text.delta","delta":"lo"}`+"\n\n"+
			`data: {"type":"response.completed","response":{"id":"r1","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hello"}]}]}}`+"\n\n")
	})
	var text strings.Builder
	var final *responses.Response
	for event, err := range svc.Stream(context.Background(), responses.Request{Model: "m", Input: responses.Text("hi")}) {
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
	if text.String() != "Hello" || final == nil || final.Text() != "Hello" {
		t.Fatalf("text %q final %+v", text.String(), final)
	}
}
