package tool_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ifitsmanu/xai-sdk-go/tool"
)

type weatherArgs struct {
	City  string `json:"city" jsonschema:"city name"`
	Units string `json:"units,omitempty"`
}

// Arguments are checked against the schema built from the struct before the function runs.
func TestFuncValidatesAndEncodes(t *testing.T) {
	ran := false
	f, err := tool.Func("weather", "Weather for a city.", func(_ context.Context, a weatherArgs) (map[string]any, error) {
		ran = true
		return map[string]any{"city": a.City, "temp": 21}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Run(context.Background(), json.RawMessage(`{"city": 7}`)); err == nil || ran {
		t.Fatalf("wrong type accepted: err=%v ran=%v", err, ran)
	}
	if _, err := f.Run(context.Background(), json.RawMessage(`{"units":"c"}`)); err == nil || ran {
		t.Fatalf("missing required field accepted: err=%v ran=%v", err, ran)
	}
	out, err := f.Run(context.Background(), json.RawMessage(`{"city":"Paris"}`))
	if err != nil || out != `{"city":"Paris","temp":21}` {
		t.Fatalf("out = %s, err = %v", out, err)
	}
	schema, _ := json.Marshal(f.Parameters)
	var s struct {
		Required   []string                     `json:"required"`
		Properties map[string]map[string]string `json:"properties"`
	}
	_ = json.Unmarshal(schema, &s)
	if len(s.Required) != 1 || s.Required[0] != "city" || s.Properties["city"]["description"] != "city name" {
		t.Fatalf("schema = %s", schema)
	}
}

// One definition, three wire shapes. These differ on the real APIs.
func TestOneToolThreeShapes(t *testing.T) {
	tools := []tool.Tool{
		&tool.Function{Name: "f"},
		tool.WebSearch{AllowedDomains: []string{"x.ai"}},
	}
	chat, err := tool.ForChat(tools)
	if err != nil || chat[0].GetFunction().GetParameters() != `{"properties":{},"type":"object"}` || chat[1].GetWebSearch().GetAllowedDomains()[0] != "x.ai" {
		t.Fatalf("chat = %v, %v", chat, err)
	}
	encode := func(v any, err error) string {
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(v)
		return string(b)
	}
	if got, want := encode(tool.ForResponses(tools)), `[{"type":"function","name":"f","parameters":{"properties":{},"type":"object"}},{"type":"web_search","filters":{"allowed_domains":["x.ai"]}}]`; got != want {
		t.Errorf("responses\n got %s\nwant %s", got, want)
	}
	if got, want := encode(tool.ForVoice(tools)), `[{"type":"function","function":{"name":"f","parameters":{"properties":{},"type":"object"}}},{"type":"web_search","allowed_domains":["x.ai"]}]`; got != want {
		t.Errorf("voice\n got %s\nwant %s", got, want)
	}
	if _, err := tool.ForVoice([]tool.Tool{tool.CodeExecution{}}); err == nil {
		t.Error("voice has no code execution tool; expected an error")
	}
}
