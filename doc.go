// Package xai is a Go client for the xAI gRPC API.
//
// Generated messages live in package xaiapi. Service helpers live in their
// own packages (chat, image, video, files, models, batch, auth, tokenize,
// embed, documents, sample). Package agent runs tool loops on top of chat.
//
// Logging uses log/slog. The client is quiet unless WithLogger or WithLogLevel
// is set. The API key is not logged.
//
// Chat sends the request it already holds. Agent tool schemas are encoded
// once, after the first successful build. The instruction is read on every turn.
package xai

// Version is the SDK version sent as the gRPC user agent.
const Version = "0.1.0"
