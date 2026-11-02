package image

import "github.com/ifitsmanu/xai-sdk-go/xaiapi"

// URL is an image addressed by a public URL or a data URL.
func URL(raw string) *xaiapi.ImageUrlContent {
	return &xaiapi.ImageUrlContent{Source: &xaiapi.ImageUrlContent_ImageUrl{ImageUrl: raw}}
}

// File is an image already stored with the Files API.
func File(id string) *xaiapi.ImageUrlContent {
	return &xaiapi.ImageUrlContent{Source: &xaiapi.ImageUrlContent_FileId{FileId: id}}
}
