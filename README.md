# ctx

[![GitHub release](https://img.shields.io/github/release/tyemirov/ctx.svg)](https://github.com/tyemirov/ctx/releases)

`ctx` helps you explore a project from the terminal. List directory trees, read source files with optional embedded
documentation, analyse call chains, or fetch documentation straight from GitHub—all with a single CLI.

## Highlights

- **One command for project discovery.** Inspect structure, source, and dependencies without leaving the shell.
- **Human-friendly or machine-ready output.** Toon (default), raw text, JSON, and XML renderers share the same data.
- **Clipboard and automation built in.** Copy results, run in MCP server mode, or integrate with scripts.
- **Optional documentation enrichment.** Embed local symbol docs or pull curated GitHub documentation alongside output.
- **Automated dependency documentation.** `ctx doc discover` scans Go/npm/Python dependencies, fetches their upstream docs, and writes curated bundles to `docs/dependencies`.

## Quick Start

Install and try the core workflows:

1. Install the latest release:

   ```shell
   go install github.com/tyemirov/ctx@latest
   ```

2. Explore a directory tree (Toon format by default):

   ```shell
   ctx tree .
   ```

3. Review file content with documentation:

   ```shell
   ctx content main.go --doc
   ```

4. Analyse a call chain:

   ```shell
   ctx callchain github.com/tyemirov/ctx/internal/commands.GetContentData
   ```

Need binaries instead? Grab pre-built releases for macOS, Linux, and Windows from the
[Releases](https://github.com/tyemirov/ctx/releases) page.

## Installation

1. Install Go ≥ 1.21.
2. Add `$GOBIN` (or `$GOPATH/bin`) to your `PATH`.
3. Run `go install github.com/tyemirov/ctx@latest`.
4. Verify with `ctx --help`.

## Core Workflows

### Explore project layout

Surface a tree for one or more paths. Toon is the default; switch formats when needed.

```shell
ctx tree repoA repoB --format raw
```

- `-e`/`--e` excludes directories by name during traversal.
- `--summary` (on by default) appends totals; disable with `--summary false`.

### Inspect files with context

Mix files and directories, embed documentation, and optionally copy results.

```shell
ctx content cmd/internal --doc --copy
```

### Analyse call chains

Trace callers and callees across Go, Python, and JavaScript.

```shell
ctx callchain fmt.Println --depth 2 --format json
```

### Fetch documentation

Render documentation from GitHub repositories or public web pages:

```shell
ctx doc --path example/project/docs --doc full
```

Provide repository coordinates (`owner/repo[/path]`) or a GitHub URL. Combine `--ref`, `--rules`, and clipboard flags as
needed. Use `--doc relevant` to limit output to referenced symbols or `--doc full` to include entire documentation
bundles. When `--path` points to a non-GitHub `https://` URL, `ctx doc` automatically switches to the web crawler; use
`--web-depth` (`0`–`3`, default `1`) to control how many same-host links are followed. Example:

```shell
ctx doc --path https://getbootstrap.com/docs/5.0/getting-started/introduction/ --web-depth 1
```

### Generate dependency documentation

Trace the dependencies used in your project and hydrate a local `docs/dependencies` tree (one Markdown file per package):

```shell
ctx doc discover . --output-dir docs/dependencies
```

- Supports Go modules (`go.mod`), npm (`package.json`), and Python (`requirements*.txt`, `pyproject.toml`) at once.
- Toggle ecosystems with `--ecosystems go,js,python`, include dev tooling via `--include-dev`, and capture indirect Go modules with `--include-indirect`.
- Emit machine-readable manifests for automation with `--format json`.
- JavaScript projects that only declare `devDependencies` automatically fall back to those packages so you still get documentation without extra flags; pass `--include-dev` to always include tooling dependencies.

Remote documentation calls support anonymous access for public repositories. Export `GH_TOKEN`, `GITHUB_TOKEN`, or `GITHUB_API_TOKEN` to authenticate when working with private sources or to raise rate limits. When targeting a custom API base (for example, a mock server in tests), any placeholder token value is sufficient.

## Helpful Flags at a Glance

```shell
ctx <tree|t|content|c|callchain|cc|doc> [arguments] [flags]
```

| Flag | Commands | What it does |
|------|----------|--------------|
| `-e, --e <pattern>` | tree, content | Skip matching child directories during traversal (repeatable). |
| `--no-gitignore`, `--no-ignore` | tree, content | Disable loading of `.gitignore`/`.ignore`. |
| `--git` | tree, content | Include the `.git` directory. |
| `--format <toon|raw|json|xml>` | all | Pick the renderer; Toon stays default. |
| `--summary <bool>` | tree, content | Toggle aggregate totals (on by default). |
| `--tokens` | tree, content | Estimate tokens (see Architecture for backend details). |
| `--model <name>` | tree, content | Choose the tokenizer model. |
| `--doc <disabled|relevant|full>` | content, callchain, doc | Control documentation enrichment. |
| `--docs-attempt` | content, callchain | Try to fetch GitHub docs for imported modules. |
| `--web-depth <0-3>` | doc | Control link traversal depth when capturing public web pages. |
| `--depth <number>` | callchain | Limit traversal depth (default `1`). |
| `--copy`, `--c` | tree, content, callchain, doc | Copy output to the clipboard after rendering. |
| `--copy-only`, `--co` | tree, content, callchain, doc | Copy without printing to stdout. |
| `--mcp` | root | Run an MCP server with JSON endpoints. |

Boolean flags accept `true/false`, `yes/no`, `on/off`, or `1/0`. Combine configuration defaults in `config.yaml` with
flag overrides to tailor each command.

## Clipboard and Copy-Only Workflows

- `--copy` mirrors terminal output to your system clipboard.
- `--copy-only` suppresses stdout while still copying, which keeps logs clean in scripted environments.
- Persist clipboard defaults by setting `copy: true` or `copy_only: true` under the relevant command in `config.yaml`.

If the current platform does not support clipboard access, commands fail fast with a descriptive error so you can fall
back to piping output.

## MCP Integration

Run `ctx --mcp` to expose commands over HTTP. The server:

- Binds to an ephemeral loopback address and prints the endpoint.
- Serves `/capabilities` (command list), `/environment` (working directory), and `/commands/<name>` (execution).
- Always returns JSON mirroring CLI output, so automation can rely on a single schema.

Use it with agents such as Claude Desktop or the Codex CLI. See `ARCHITECTURE.md` for configuration snippets and
advanced guidance.

## Configuration and Ignore Rules

- `.ignore` and `.gitignore` files are respected at every directory root during traversal.
- Explicit file arguments are never filtered.
- Use the repeatable `-e/--e` flag to skip direct child directories by name.
- Create `config.yaml` to store per-command defaults (format, documentation mode, summary, clipboard, etc.).

## Token Counting and Binary Files

- Add `--tokens` to tree or content commands to track token estimates alongside file counts and sizes.
- Choose alternative models with `--model <name>`; details on supported backends live in `ARCHITECTURE.md`.
- Raw output omits binary payloads by default. Add a `[binary]` section to `.ignore` to include base64 content in JSON
  and XML outputs.

## Examples

Display a raw tree view excluding `dist` folders:

```shell
ctx tree projectA projectB -e dist --format raw
```

Copy Toon output without printing to stdout:

```shell
ctx content pkg --doc --copy-only
```

Analyse a call chain in XML with embedded docs:

```shell
ctx callchain github.com/tyemirov/ctx/internal/commands.GetContentData --depth 2 --doc --format xml
```

## Need More Detail?

- [ARCHITECTURE.md](ARCHITECTURE.md) explains internal design, streaming, token backends, MCP wiring, and release steps.
- [POLICY.md](POLICY.md) documents confident-programming rules for contributors.
- [PRD.md](PRD.md) captures product requirements and behaviour expectations.
- [CHANGELOG.md](CHANGELOG.md) lists release history and pending changes.

## License

ctx is released under the [MIT License](MIT-LICENSE).
