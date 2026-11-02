package chat

import "github.com/ifitsmanu/xai-sdk-go/xaiapi"

// Response is a chat completion.
type Response struct {
	Proto *xaiapi.GetChatCompletionResponse
	bufs  []streamBuf
}

// ID is the response id used for billing and stored-message lookups.
func (r *Response) ID() string {
	if r == nil || r.Proto == nil {
		return ""
	}
	return r.Proto.Id
}

// Model is the concrete model that served the request.
func (r *Response) Model() string {
	if r == nil || r.Proto == nil {
		return ""
	}
	return r.Proto.Model
}

// Content is the text of the last assistant output.
func (r *Response) Content() string {
	msg, idx := r.lastAssistant()
	if msg == nil {
		return ""
	}
	if idx < len(r.bufs) && r.bufs[idx].contentLive {
		return string(r.bufs[idx].content)
	}
	return msg.Content
}

// Reasoning is the reasoning trace of the last assistant output.
func (r *Response) Reasoning() string {
	msg, idx := r.lastAssistant()
	if msg == nil {
		return ""
	}
	if idx < len(r.bufs) && r.bufs[idx].reasoningLive {
		return string(r.bufs[idx].reasoning)
	}
	return msg.ReasoningContent
}

// ToolCalls lists every tool call across outputs.
func (r *Response) ToolCalls() []*xaiapi.ToolCall {
	if r == nil || r.Proto == nil {
		return nil
	}
	var calls []*xaiapi.ToolCall
	for _, output := range r.Proto.Outputs {
		if output == nil || output.Message == nil {
			continue
		}
		calls = append(calls, output.Message.ToolCalls...)
	}
	return calls
}

// Choices is one text per output, in API order. SampleN uses this when n is greater than one.
func (r *Response) Choices() []string {
	if r == nil || r.Proto == nil {
		return nil
	}
	out := make([]string, 0, len(r.Proto.Outputs))
	for _, output := range r.Proto.Outputs {
		text := ""
		if output != nil && output.Message != nil {
			text = output.Message.Content
		}
		out = append(out, text)
	}
	return out
}

// Citations lists URLs the model used when search was enabled.
func (r *Response) Citations() []string {
	if r == nil || r.Proto == nil {
		return nil
	}
	return r.Proto.Citations
}

func (r *Response) lastAssistant() (*xaiapi.CompletionMessage, int) {
	if r == nil || r.Proto == nil {
		return nil, -1
	}
	var last *xaiapi.CompletionMessage
	idx := -1
	for i, output := range r.Proto.Outputs {
		if output == nil || output.Message == nil {
			continue
		}
		if output.Message.Role == xaiapi.MessageRole_ROLE_ASSISTANT {
			last = output.Message
			idx = i
		}
	}
	return last, idx
}

// streamBuf grows reply text without copying the whole string on every token.
type streamBuf struct {
	content       []byte
	reasoning     []byte
	encrypted     []byte
	contentLive   bool
	reasoningLive bool
	encryptedLive bool
}

// ApplyChunk folds one streaming chunk into the response.
func (r *Response) ApplyChunk(chunk *xaiapi.GetChatCompletionChunk) {
	if chunk == nil || r == nil || r.Proto == nil {
		return
	}
	p := r.Proto
	if chunk.Usage != nil {
		p.Usage = chunk.Usage
	}
	if chunk.Created != nil {
		p.Created = chunk.Created
	}
	if chunk.Id != "" {
		p.Id = chunk.Id
	}
	if chunk.Model != "" {
		p.Model = chunk.Model
	}
	if chunk.SystemFingerprint != "" {
		p.SystemFingerprint = chunk.SystemFingerprint
	}
	if chunk.ServiceTier != xaiapi.ServiceTier_SERVICE_TIER_UNSPECIFIED {
		p.ServiceTier = chunk.ServiceTier
	}
	p.Citations = append(p.Citations, chunk.Citations...)
	if len(chunk.OutputFiles) > 0 {
		p.OutputFiles = append(p.OutputFiles, chunk.OutputFiles...)
	}

	maxIndex := int32(-1)
	for _, c := range chunk.Outputs {
		if c != nil && c.Index > maxIndex {
			maxIndex = c.Index
		}
	}
	for int32(len(p.Outputs)) <= maxIndex {
		p.Outputs = append(p.Outputs, &xaiapi.CompletionOutput{Message: &xaiapi.CompletionMessage{}})
	}
	for _, c := range chunk.Outputs {
		if c == nil {
			continue
		}
		choice := p.Outputs[c.Index]
		choice.Index = c.Index
		if choice.Message == nil {
			choice.Message = &xaiapi.CompletionMessage{}
		}
		if c.Delta != nil {
			if c.Delta.Role != xaiapi.MessageRole_INVALID_ROLE {
				choice.Message.Role = c.Delta.Role
			}
			r.appendLive(int(c.Index), choice.Message, c.Delta)
			choice.Message.ToolCalls = append(choice.Message.ToolCalls, c.Delta.ToolCalls...)
			choice.Message.Citations = append(choice.Message.Citations, c.Delta.Citations...)
		}
		if c.FinishReason != xaiapi.FinishReason_REASON_INVALID {
			choice.FinishReason = c.FinishReason
		}
	}
}

func (r *Response) appendLive(index int, msg *xaiapi.CompletionMessage, delta *xaiapi.Delta) {
	if delta.Content == "" && delta.ReasoningContent == "" && delta.EncryptedContent == "" {
		return
	}
	for len(r.bufs) <= index {
		r.bufs = append(r.bufs, streamBuf{})
	}
	b := &r.bufs[index]
	if delta.Content != "" {
		if !b.contentLive {
			b.content = append(b.content, msg.Content...)
			b.contentLive = true
		}
		b.content = append(b.content, delta.Content...)
	}
	if delta.ReasoningContent != "" {
		if !b.reasoningLive {
			b.reasoning = append(b.reasoning, msg.ReasoningContent...)
			b.reasoningLive = true
		}
		b.reasoning = append(b.reasoning, delta.ReasoningContent...)
	}
	if delta.EncryptedContent != "" {
		if !b.encryptedLive {
			b.encrypted = append(b.encrypted, msg.EncryptedContent...)
			b.encryptedLive = true
		}
		b.encrypted = append(b.encrypted, delta.EncryptedContent...)
	}
}

// Flush copies buffered stream text into the protobuf once.
func (r *Response) Flush() {
	if r == nil || r.Proto == nil || len(r.bufs) == 0 {
		return
	}
	for i, b := range r.bufs {
		if i >= len(r.Proto.Outputs) || r.Proto.Outputs[i] == nil || r.Proto.Outputs[i].Message == nil {
			continue
		}
		msg := r.Proto.Outputs[i].Message
		if b.contentLive {
			msg.Content = string(b.content)
		}
		if b.reasoningLive {
			msg.ReasoningContent = string(b.reasoning)
		}
		if b.encryptedLive {
			msg.EncryptedContent = string(b.encrypted)
		}
	}
	r.bufs = nil
}
