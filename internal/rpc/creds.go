package rpc

import "context"

type apiKeyCredentials struct {
	key      string
	metadata map[string]string
	insecure bool
}

func (a apiKeyCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	md := make(map[string]string, len(a.metadata)+1)
	for k, v := range a.metadata {
		md[k] = v
	}
	md["authorization"] = "Bearer " + a.key
	return md, nil
}

func (a apiKeyCredentials) RequireTransportSecurity() bool { return !a.insecure }
