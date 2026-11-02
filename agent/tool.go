package agent

import (
	"context"
	"encoding/json"

	"github.com/ifitsmanu/xai-sdk-go/tool"
)

// DoneTool is the name of the tool Done returns.
const DoneTool = "done"

// Done is a tool the model calls when the task is finished. RunLoop stops on
// its result when given IsDone.
func Done() *tool.Function {
	return &tool.Function{
		Name:        DoneTool,
		Description: "Call this when the task is finished.",
		Run:         func(context.Context, json.RawMessage) (string, error) { return DoneTool, nil },
	}
}

// IsDone reports whether e is the result of the Done tool.
func IsDone(e Event) bool { return e.Kind == KindToolResult && e.Tool == DoneTool }
