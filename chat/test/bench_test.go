package chat_test

import (
	"testing"

	"github.com/ifitsmanu/xai-sdk-go/chat"
	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
)

func BenchmarkStreamAccumulate(b *testing.B) {
	chunk := &xaiapi.GetChatCompletionChunk{
		Outputs: []*xaiapi.CompletionOutputChunk{{
			Delta: &xaiapi.Delta{Role: xaiapi.MessageRole_ROLE_ASSISTANT, Content: "token "},
		}},
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		resp := &chat.Response{Proto: &xaiapi.GetChatCompletionResponse{}}
		for n := 0; n < 256; n++ {
			resp.ApplyChunk(chunk)
		}
		if resp.Content() == "" {
			b.Fatal("empty")
		}
		resp.Flush()
	}
}
