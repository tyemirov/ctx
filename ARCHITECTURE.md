# Architecture

This document captures the internal structure of `ctx`, the command-line tool for navigating projects, inspecting source
files, and analysing call chains. The README focuses on user-facing flows; use this guide when you need to understand
how the CLI is assembled or when you are extending the codebase.

## Package Layout

- `main.go`: module-root entry point so `go install github.com/tyemirov/ctx@latest` and `go run ./...` both invoke the same binary.
- `cmd/ctx`: CLI bootstrap invoked by `main.go`; builds the zap logger and delegates to the CLI layer.
- `internal/cli`: Cobra command wiring, configuration loading, boolean flag normalisation, clipboard control, MCP server
  bootstrap, and documentation orchestration.
- `internal/commands`: filesystem traversal (`TreeBuilder`, `StreamTree`), content streaming (`StreamContent`), and
  collectors that build output DTOs.
- `internal/services/stream`: shared event protocol for directory, file, content chunk, summary, warning, and error
  events.
- `internal/output`: Toon, JSON, XML, and raw renderers that subscribe to stream events.
- `internal/docs` and `internal/docs/githubdoc`: documentation collector plus GitHub fetcher used by `ctx doc` and docs
  attempts.
- `internal/docs/webdoc`: HTML crawler/sanitizer used by `ctx doc` whenever `--path` points to a non-GitHub HTTPS URL, providing depth-limited same-host extraction.
- `internal/discover`: dependency detectors (Go, JavaScript, Python), registry clients, and documentation writers used by `ctx doc discover`.
- `internal/callchain`: analyser registry and language-specific call-chain analysers for Go, Python, and JavaScript.
- `internal/tokenizer`: token counting backends, helper process orchestration, and CLI adapters.
- `internal/config`: configuration discovery/merging and `ctx --init` scaffolding.
- `internal/types`: shared structs for tree nodes, file outputs, call chains, documentation entries, and Toon payloads.
- `internal/utils`: logging helpers, mime sniffing, ignore pattern utilities, time formatting, and shared constants.
- `internal/services/mcp`: HTTP server exposing the CLI over Model Context Protocol.
- `internal/services/clipboard`: platform-specific clipboard abstraction.
- `tests/`: black-box integration flows covering CLI streaming, MCP, and documentation scenarios.

## Runtime Overview

- `main.go` calls `cmd/ctx.Run`, which constructs a console zap logger via `utils.NewApplicationLogger` and invokes `cli.Execute()`.
- `internal/cli` builds the Cobra root command, registers subcommands, and applies configuration defaults once per run.
  It normalises boolean flags (`boolean_flag.go`), coordinates streaming via `errgroup.Group`, and owns clipboard
  dispatch after rendering completes.
- `internal/commands` resolves filesystem inputs, applies ignore rules, and emits streaming events consumed by
  renderers. Call-chain requests are delegated to the analyser registry.
- `internal/services/stream` defines the event schema and helper functions for building tree/content streams.
- `internal/output` renders events to Toon, JSON, XML, or raw text while keeping the schemas aligned.
- `internal/docs` orchestrates documentation collection from local analysis and GitHub fetches.
- `internal/discover` scans dependency manifests, resolves repositories via npm/PyPI metadata, fetches upstream documentation through the shared GitHub client, and writes Markdown bundles plus manifests for `ctx doc discover`.
- `internal/tokenizer` chooses a token backend based on the configured model and runs helper processes when required.
- `internal/services/mcp` wraps the CLI surface in an HTTP server for MCP clients.

## Command Pipeline

1. Cobra resolves the selected command, merges configuration defaults with CLI flags, and constructs a request.
2. The CLI layer initialises a streaming session (`stream.Session`) and renderer pair governed by an `errgroup.Group`.
3. `internal/commands` walks the filesystem (tree/content) or invokes call-chain analysers, emitting events as work
   completes. Warnings propagate via the stream without aborting execution.
4. Renderers in `internal/output` consume events incrementally and write formatted output to the command’s writer.
5. Post-processing hooks (clipboard copy, summaries, token totals) run after renders succeed.

The pipeline is streaming end-to-end; directory traversal and content emission never buffer entire trees before
producing output. This design keeps large projects responsive and guarantees JSON/XML/Toon/raw stay in sync.

## Tree and Content Commands

- `TreeBuilder` configures traversal (ignore patterns, summary toggle, token model) and invokes `StreamTree`. Events are
  aggregated by `treeCollector`, which builds `types.TreeOutputNode` graphs and applies summaries with `applySummary`.
- `StreamTree` emits directory enter/leave events, file metadata, warnings, and summaries. Errors stop the traversal and
  bubble up to the CLI.
- `StreamContent` emits `types.FileOutput` values and chunk events for renderers. Binary detection uses
  `utils.ShouldTreatAsBinary`, and token counting hooks into `tokenizer.Counter` implementations.
- Token counting failures surface as warnings (`WarningTokenCountFormat`) so the rest of the traversal can continue.
- Binary payloads obey `[binary]` sections in `.ignore`; otherwise only metadata is produced for binary files.

## Documentation Retrieval

`ctx doc` consumes repository coordinates (`--path`, optionally `--ref`) and streams directory entries via the GitHub
Contents API. Cleanup rules (`--rules`) filter files before rendering.

The same fetcher powers `--docs-attempt` on `content` and `callchain`: imports are analysed, GitHub repositories are
inferred, and documentation bundles are attached to the output.

`docs.Collector` merges local symbol analysis with remote content and emits `types.DocumentationEntry` slices. Renderers
attach these entries to files and call-chain nodes when documentation modes demand it.

Documentation mode (`--doc disabled|relevant|full`) controls what gets embedded: `relevant` includes only locally
referenced symbols; `full` augments with GitHub documentation bundles.

## Dependency Documentation Discovery

`ctx doc discover` reuses the same GitHub fetcher but adds a discovery layer:

- `internal/discover/detector_go` parses `go.mod`, filters indirect modules (unless `--include-indirect`), and maps GitHub
  module paths to repositories.
- `internal/discover/detector_js` reads `package.json`, queries the npm registry (override with `--npm-registry-base`),
  and extracts GitHub repositories from package metadata. Dev dependencies opt in via `--include-dev`.
- `internal/discover/detector_python` parses `requirements*.txt` and `pyproject.toml`, resolves projects through the
  PyPI JSON API (override with `--pypi-registry-base`), and maps `project_urls` to GitHub.
- `discover.Runner` fans out over dependencies with bounded concurrency, fetches documentation from GitHub (respecting
  `--rules` cleanup and hidden `--api-base` overrides), and writes Markdown bundles to `docs/dependencies/<ecosystem>/`.
- A manifest (text summary or `--format json`) records status, output paths, and failure reasons so automation can
  consume the results or feed them into CI.

## Call Chain Analysis

- The CLI builds an `internal/callchain.Registry` composed of Go, Python, and JavaScript analysers.
- Each analyser satisfies the `Analyzer` interface, traverses language-specific graphs, and returns
  `types.CallChainOutput` with callers, callees, function sources, and documentation references.
- The registry asks analysers in order; the first analyser to resolve the symbol wins. Missing symbols fall back to
  `callchain.ErrSymbolNotFound`, which bubbles to the CLI and the MCP surface.
- Requests include a `docs.Collector`, allowing analyser results to share the same documentation pipeline used by
  `content`.
- Depth (`--depth`) and documentation flags flow directly from configuration or CLI input.

## Token Counting Services

Token counting is optional and enabled with `--tokens`. When active, directory summaries accumulate file counts, byte
 totals, and token estimates. `internal/tokenizer` chooses the backend from the model prefix:

| Prefix | Backend | Notes |
|--------|---------|-------|
| `gpt-`, `text-embedding`, `davinci`, `curie`, `babbage`, `ada`, `code-` | OpenAI encoders via `tiktoken-go`; falls back to `cl100k_base` when encodings are missing. |
| `claude-` | Anthropic helper launched with [`uv`](https://github.com/astral-sh/uv); requires `ANTHROPIC_API_KEY`. |
| `llama-` | SentencePiece helper launched with `uv`; downloads a compatible tokenizer model on demand. |
| other | Defaults to `cl100k_base`. |

Set `CTX_UV` to override the helper executable. Integration tests can exercise helpers with
`CTX_TEST_PYTHON=python3 go test -tags python_helpers ./internal/tokenizer`. Set `CTX_TEST_RUN_HELPERS=1` to enable the
optional helper suite and `CTX_TEST_UV` to point at a custom `uv` binary.

## Binary File Handling

- When `.ignore` has no `[binary]` section, binary files are omitted from raw output. JSON/XML still surface `mimeType`
  metadata.
- Add a `[binary]` section listing glob patterns to emit base64-encoded payloads. Matched files appear as `type:
  "binary"` with inline content.
- Explicitly listed files are never filtered by ignore rules; traversal filters apply only to recursive directory walks.

## Output Rendering

Streaming events cover directory entry/exit, file metadata, content chunks, summaries, warnings, and errors. Renderers
map events to their respective formats:

- **Raw:** Human-readable tree blocks (`[File] path`) and inline summaries.
- **JSON:** Newline-delimited JSON events matching the schema in `internal/services/stream/events.go`.
- **XML:** `<events><event …></event></events>` envelopes with XML-serialised events.
- **Toon:** Structured Toon documents with sections for metadata, files, summaries, and documentation.

Downstream tools can rebuild aggregate views from the same event feed without extra branching in the CLI or renderers.

Run the streaming regression suite with:

```bash
go test ./internal/services/stream ./internal/output ./internal/cli
```

These tests verify event ordering and renderer behaviour.

## Configuration System

- Global configuration lives at `$HOME/.ctx/config.yaml`; local configuration defaults to `<project>/config.yaml`.
- `config.LoadApplicationConfiguration` merges global, then local, then command-line overrides. `normalizeCopySettings`
  keeps legacy `clipboard` fields compatible with `copy`/`copy_only`.
- The `doc_discover` block configures discovery defaults (output dir, ecosystems, include/exclude patterns, concurrency,
  registry overrides, and clipboard preferences) so CI pipelines can run `ctx doc discover` without flags.
- `config.InitializeConfiguration` (`ctx --init [local|global]`) scaffolds a template configuration and honours
  `--force`.
- `config.StreamCommandConfiguration` stores per-command defaults for format, documentation mode, docs attempts, token
  counting, clipboard preferences, and path exclusions. `utils.DeduplicatePatterns` removes repeated patterns after
  merges.
- `.ignore` and `.gitignore` files are honoured at every directory root during traversal, while explicit file arguments
  bypass ignore filters entirely. The repeatable `-e/--e` flag excludes direct child directories by name.

## MCP Server

The root `--mcp` flag launches an HTTP server that advertises capabilities at `/capabilities`, reports its working
 directory via `/environment`, and executes commands through `/commands/<name>`. Responses are always JSON, mirroring the
 equivalent CLI output. The server listens on `127.0.0.1:0` and prints the bound address. Shutdown waits up to five
 seconds for in-flight requests.

`internal/cli/mcp_executor.go` adapts CLI commands into MCP executors. Requests are unmarshalled into command-specific
payloads, executed through the same streaming pipeline, and wrapped in `mcp.CommandResponse`. Errors propagate via
`mcp.CommandExecutionError`, letting HTTP handlers respond with contextual status codes and JSON `{"error": "…"}`
 bodies.

## Clipboard Integration

`--copy` (alias `--c`) mirrors rendered output to the system clipboard after the renderer completes. `--copy-only` (alias
`--co`) skips writing to stdout while keeping clipboard behaviour—ideal for piping results into other tools. Clipboard
requests fail fast with contextual errors when the platform is unsupported. Persistent defaults live in `config.yaml`
 under the relevant command.

## Error Handling and Logging

- All command execution flows return `error`; the CLI wraps failures with contextual `fmt.Errorf("…: %w")` messages.
- `cmd/ctx.Run` panics only when the zap logger cannot be initialised. Runtime errors render as fatal log entries with
  `utils.ApplicationExecutionFailedMessage`.
- Streaming pipelines capture warning events (missing files, token counting failures, binary skips) without halting the
  command.
- The MCP surface returns structured JSON errors by wrapping failures in `CommandExecutionError` with HTTP status codes.

## External Dependencies

- [`spf13/cobra`](https://github.com/spf13/cobra) for CLI parsing.
- [`spf13/viper`](https://github.com/spf13/viper) for configuration discovery.
- [`uber-go/zap`](https://github.com/uber-go/zap) for console logging.
- [`golang.org/x/sync/errgroup`](https://pkg.go.dev/golang.org/x/sync/errgroup) to coordinate streaming pipelines.
- [`tiktoken-go`](https://github.com/pkoukk/tiktoken-go) and helper scripts executed via [`uv`](https://github.com/astral-sh/uv)
  for token counting.
- Standard library `net/http`, `encoding/json`, and `encoding/xml` for MCP hosting and renderer output.

## Testing Strategy

- Unit tests cover configuration merging, boolean flag normalisation, streaming event ordering, and MCP executor error
  handling.
- Integration tests in `internal/cli` and `tests/` launch real commands (including `ctx --mcp`) to validate end-to-end
  streaming, documentation enrichment, and clipboard toggles.
- Tokenizer tests optionally run helper scripts when the requisite environment variables are present, ensuring
  deterministic coverage when dependencies exist.
- All suites run via `go test ./...`. Tests use `t.TempDir()` to avoid filesystem pollution.

## Release Workflow

Follow these steps to publish a tagged release:

1. Update `CHANGELOG.md` with a new version section.
2. Commit the changelog update.
3. Tag the commit and push both the branch and the tag:

   ```bash
   git tag vX.Y.Z
   git push origin master
   git push origin vX.Y.Z
   ```

Tags starting with `v` trigger the automated release workflow, build platform binaries, and extract release notes from
the matching changelog section.
