package batch

import "github.com/ifitsmanu/xai-sdk-go/xaiapi"

// Completion wraps a chat request for AddRequests. id must be unique inside the batch when set.
func Completion(id string, req *xaiapi.GetCompletionsRequest) *xaiapi.BatchRequest {
	return &xaiapi.BatchRequest{
		BatchRequestId: idPtr(id),
		Request:        &xaiapi.BatchRequest_CompletionRequest{CompletionRequest: req},
	}
}

// Image wraps an image request for AddRequests.
func Image(id string, req *xaiapi.GenerateImageRequest) *xaiapi.BatchRequest {
	return &xaiapi.BatchRequest{
		BatchRequestId: idPtr(id),
		Request:        &xaiapi.BatchRequest_ImageRequest{ImageRequest: req},
	}
}

// Video wraps a video generation request for AddRequests.
func Video(id string, req *xaiapi.GenerateVideoRequest) *xaiapi.BatchRequest {
	return &xaiapi.BatchRequest{
		BatchRequestId: idPtr(id),
		Request:        &xaiapi.BatchRequest_VideoRequest{VideoRequest: req},
	}
}

// Extend wraps a video extension request for AddRequests.
func Extend(id string, req *xaiapi.ExtendVideoRequest) *xaiapi.BatchRequest {
	return &xaiapi.BatchRequest{
		BatchRequestId: idPtr(id),
		Request:        &xaiapi.BatchRequest_VideoExtensionRequest{VideoExtensionRequest: req},
	}
}

func idPtr(id string) *string {
	if id == "" {
		return nil
	}
	return &id
}
