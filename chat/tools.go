package chat

import "github.com/ifitsmanu/xai-sdk-go/xaiapi"

// ToolChoiceMode forces auto, none, or required. Declare tools with package tool.
func ToolChoiceMode(mode xaiapi.ToolMode) *xaiapi.ToolChoice {
	return &xaiapi.ToolChoice{ToolChoice: &xaiapi.ToolChoice_Mode{Mode: mode}}
}

// RequiredFunction forces a call to the named function.
func RequiredFunction(name string) *xaiapi.ToolChoice {
	return &xaiapi.ToolChoice{ToolChoice: &xaiapi.ToolChoice_FunctionName{FunctionName: name}}
}
