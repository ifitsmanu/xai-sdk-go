package chat

import "github.com/ifitsmanu/xai-sdk-go/xaiapi"

// Text is a text content part.
func Text(s string) *xaiapi.Content {
	return &xaiapi.Content{Content: &xaiapi.Content_Text{Text: s}}
}

// ImageURL is an image content part addressed by URL or a data URL.
func ImageURL(url string) *xaiapi.Content {
	return &xaiapi.Content{Content: &xaiapi.Content_ImageUrl{ImageUrl: &xaiapi.ImageUrlContent{
		Source: &xaiapi.ImageUrlContent_ImageUrl{ImageUrl: url},
	}}}
}

// FileID attaches a file that was uploaded with the Files API.
func FileID(id string) *xaiapi.Content {
	return &xaiapi.Content{Content: &xaiapi.Content_File{File: &xaiapi.FileContent{FileId: id}}}
}

// FileBytes attaches inline file bytes. Set mime when the filename is not enough to infer it.
func FileBytes(filename, mime string, data []byte) *xaiapi.Content {
	return &xaiapi.Content{Content: &xaiapi.Content_File{File: &xaiapi.FileContent{
		Data:     data,
		Filename: filename,
		MimeType: mime,
	}}}
}

// FileURL attaches a file fetched from a public URL.
func FileURL(url string) *xaiapi.Content {
	return &xaiapi.Content{Content: &xaiapi.Content_File{File: &xaiapi.FileContent{Url: url}}}
}

// User builds a user message from content parts.
func User(parts ...*xaiapi.Content) *xaiapi.Message {
	return message(xaiapi.MessageRole_ROLE_USER, parts)
}

// System builds a system message.
func System(parts ...*xaiapi.Content) *xaiapi.Message {
	return message(xaiapi.MessageRole_ROLE_SYSTEM, parts)
}

// Assistant builds an assistant message.
func Assistant(parts ...*xaiapi.Content) *xaiapi.Message {
	return message(xaiapi.MessageRole_ROLE_ASSISTANT, parts)
}

// Developer builds a developer message.
func Developer(parts ...*xaiapi.Content) *xaiapi.Message {
	return message(xaiapi.MessageRole_ROLE_DEVELOPER, parts)
}

// UserText is User(Text(s)).
func UserText(s string) *xaiapi.Message { return User(Text(s)) }

// SystemText is System(Text(s)).
func SystemText(s string) *xaiapi.Message { return System(Text(s)) }

// ToolResult is a tool response. toolCallID pairs it with the model's call when set.
func ToolResult(result, toolCallID string) *xaiapi.Message {
	msg := &xaiapi.Message{
		Role:    xaiapi.MessageRole_ROLE_TOOL,
		Content: []*xaiapi.Content{Text(result)},
	}
	if toolCallID != "" {
		msg.ToolCallId = &toolCallID
	}
	return msg
}

func message(role xaiapi.MessageRole, parts []*xaiapi.Content) *xaiapi.Message {
	return &xaiapi.Message{Role: role, Content: parts}
}

// MessagesFromResponse turns a completion into input messages.
// A tool-role output copies the first tool call id onto the message so the
// server can pair the replayed turn with the call that produced it.
func MessagesFromResponse(resp *Response) []*xaiapi.Message {
	if resp == nil || resp.Proto == nil {
		return nil
	}
	out := make([]*xaiapi.Message, 0, len(resp.Proto.Outputs))
	for _, output := range resp.Proto.Outputs {
		if output == nil || output.Message == nil {
			continue
		}
		src := output.Message
		msg := &xaiapi.Message{
			Role:             src.Role,
			EncryptedContent: src.EncryptedContent,
			ToolCalls:        src.ToolCalls,
		}
		if src.Content != "" {
			msg.Content = []*xaiapi.Content{Text(src.Content)}
		}
		if src.ReasoningContent != "" {
			reasoning := src.ReasoningContent
			msg.ReasoningContent = &reasoning
		}
		if src.Role == xaiapi.MessageRole_ROLE_TOOL && len(src.ToolCalls) > 0 && src.ToolCalls[0] != nil {
			id := src.ToolCalls[0].Id
			msg.ToolCallId = &id
		}
		out = append(out, msg)
	}
	return out
}

// ClientFunction returns the function payload when the model expects the client to run it.
func ClientFunction(tc *xaiapi.ToolCall) (*xaiapi.FunctionCall, bool) {
	if tc == nil {
		return nil, false
	}
	fn := tc.GetFunction()
	if fn == nil || fn.Name == "" {
		return nil, false
	}
	switch tc.Type {
	case xaiapi.ToolCallType_TOOL_CALL_TYPE_INVALID, xaiapi.ToolCallType_TOOL_CALL_TYPE_CLIENT_SIDE_TOOL:
	default:
		return nil, false
	}
	switch tc.Status {
	case xaiapi.ToolCallStatus_TOOL_CALL_STATUS_FAILED, xaiapi.ToolCallStatus_TOOL_CALL_STATUS_INCOMPLETE:
		return nil, false
	default:
		return fn, true
	}
}
