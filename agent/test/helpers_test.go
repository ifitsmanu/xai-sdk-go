package agent_test

import (
	"context"
	"encoding/json"

	"github.com/ifitsmanu/xai-sdk-go/tool"
)

func fn(name, description string, params any, run func(context.Context, json.RawMessage) (string, error)) *tool.Function {
	return &tool.Function{Name: name, Description: description, Parameters: params, Run: run}
}
