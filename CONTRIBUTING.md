# Contributing

This is an independent open-source Go client for the **xAI API** (`api.x.ai`). It is not published by xAI. The official SDK is the Python one.

The name in code, docs, and commits is the xAI API. The key is `XAI_API_KEY`. Generated messages are package `xaiapi`. The module path `github.com/ifitsmanu/xai-sdk-go` uses Manu's GitHub account. That account name is not part of the API.

## Before a push

```bash
make check
```

That formats nothing by itself. It fails if `gofmt` would change a file, then lints and runs the tests. Fix formatting with `make fmt`.

## Pull requests

Branch from `main`. Open a pull request into `main`. The form is `.github/pull_request_template.md`.

The `check` workflow runs `make check` on the pull request. Merge when that check is green. Delete the branch after the merge.

Tag a release from `main` after the merge. Do not tag a feature branch.

## What gets committed

Commit Go source, tests, examples, `xaiapi/` (generated, so users do not need `buf`), `go.mod`, `go.sum`, `LICENSE`, `README.md`, and `Makefile`. `.gitignore` lists what stays out. Do not commit API keys.

`xaiapi/` changes only through `make generate`, from a sibling checkout of `xai-proto`. Do not hand-edit those files.

## Tests

Tests sit next to the code they cover. `chat/chat_test.go` tests package `chat`. A test does not call the network and does not need `XAI_API_KEY`.

Add one:

```go
func TestThing(t *testing.T) {
    if got := Thing(); got != "ok" {
        t.Fatalf("got %q", got)
    }
}
```

Then `make test`.

Remove one by deleting that function. If the file has no tests left, delete the file too, then `make test` again.

Benchmarks are `func BenchmarkName(b *testing.B)` in a `bench_test.go` file. `make bench` runs them. They are not part of `make test`.

## Version

`Version` in `doc.go` is the SDK version and the gRPC user agent. Tag the same number: `v0.1.0`. `go get` installs the tag.

On `v0`, breaking changes stay on `github.com/ifitsmanu/xai-sdk-go`. `v1.0.0` stays on that path too. `/v2` starts only when `v1` itself breaks.

## Lint

`make fmt` runs `gofmt`. `make lint` checks that, then runs `golangci-lint`.
