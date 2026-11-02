package video

import "github.com/ifitsmanu/xai-sdk-go/xaiapi"

// URL is a video addressed by a public URL or a data URL.
func URL(raw string) *xaiapi.VideoUrlContent {
	return &xaiapi.VideoUrlContent{Source: &xaiapi.VideoUrlContent_Url{Url: raw}}
}

// File is a video already stored with the Files API.
func File(id string) *xaiapi.VideoUrlContent {
	return &xaiapi.VideoUrlContent{Source: &xaiapi.VideoUrlContent_FileId{FileId: id}}
}

// Voice selects a preset voice for reference-to-video. The id is the same one text-to-speech uses.
func Voice(id string) *xaiapi.AudioUrlContent {
	return &xaiapi.AudioUrlContent{Source: &xaiapi.AudioUrlContent_VoiceId{VoiceId: id}}
}
