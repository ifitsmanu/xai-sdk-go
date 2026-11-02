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

// Store asks the API to keep the generated file. expiresAfter is seconds; zero means no expiry.
// Set public on the returned options when the file should also have a public URL.
func Store(filename string, expiresAfter int64) *xaiapi.StorageOptions {
	opt := &xaiapi.StorageOptions{Filename: filename}
	if expiresAfter > 0 {
		opt.ExpiresAfter = &expiresAfter
	}
	return opt
}
