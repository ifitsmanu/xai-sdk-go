package agent

import (
	"context"
	"encoding/json"

	"github.com/ifitsmanu/xai-sdk-go/tool"
)

// Done is a tool the model calls to end a loop early. Running it sets stop.
func Done(stop *bool) *tool.Function {
	return &tool.Function{
		Name:        "done",
		Description: "Call this when the task is finished and no further tool calls are needed.",
		Run: func(context.Context, json.RawMessage) (string, error) {
			if stop != nil {
				*stop = true
			}
			return "done", nil
		},
	}
}
