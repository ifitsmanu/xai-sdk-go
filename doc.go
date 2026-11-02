// Package xai is an independent Go client for the xAI API at api.x.ai.
// It is written by Manu and is not published by xAI.
//
// Generated messages live in package xaiapi. Chat, imagine, files, models,
// and batch are separate APIs on one client. Package agent runs tool loops
// on top of chat.
//
// Responses, skills, account, and voice are HTTP.
// Collections, billing, audit, and team keys use a management key.
//
// Logging uses log/slog. The client is quiet unless WithLogger or WithLogLevel
// is set. The API key is not logged.
//
// Chat sends the request it already holds. Agent tool schemas are encoded
// once, after the first successful build. The instruction is read on every turn.
package xai

// Version is the SDK version sent as the gRPC user agent.
const Version = "0.1.0"
