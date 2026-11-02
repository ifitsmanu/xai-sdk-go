package batch_test

import (
	"testing"

	"github.com/ifitsmanu/xai-sdk-go/batch"
)

func TestCompletionOmitsEmptyID(t *testing.T) {
	req := batch.Completion("", nil)
	if req.BatchRequestId != nil {
		t.Fatal("empty id was set")
	}
	req = batch.Completion("one", nil)
	if req.GetBatchRequestId() != "one" || req.GetCompletionRequest() != nil {
		t.Fatalf("request = %+v", req)
	}
}
