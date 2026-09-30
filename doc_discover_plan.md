# ctx `doc discover` Implementation Plan (CT-100)

## Objectives

- Introduce `ctx doc discover`, a subcommand that inspects a project, identifies its major dependencies across supported ecosystems, and writes curated documentation bundles into a local `docs/dependencies` tree.
- Reuse existing streaming/documentation infrastructure (`internal/docs`, `internal/docs/githubdoc`, `internal/output`) while adding a discovery layer that can evolve independently.
- Keep the plan implementation-ready for CT-101 by outlining UX, architecture, heuristics, validation, and testing.

## Scope and Constraints

- **In scope**
  - Local project analysis (default `.` or `--path <dir>`).
  - Dependency discovery for Go (via `go.mod`/`go.sum`), JavaScript/TypeScript/CSS (via `package.json`, import maps, CDN manifests), and Python (via `pyproject.toml`/`requirements.txt`).
  - Mapping dependencies to canonical documentation sources (prefer GitHub repositories, fall back to official doc sites).
  - Fetching documentation with concurrent HTTP calls, applying cleanup rules, and persisting Markdown files under `docs/dependencies/<ecosystem>/<name>.md`.
  - Configuration and flag plumbing so discovery rules are fully deterministic and reproducible.
- **Out of scope for CT-100**
  - Actual implementation, CLI wiring, or tests (deferred to CT-101).
  - Additional ecosystems (Rust, Java, etc.) beyond the ones listed above; they will be staged later once detectors exist.

## CLI and Configuration Design

- Command shape: `ctx doc discover [PATH]`.
  - `PATH` defaults to `.`.
  - Flags:
    - `--output-dir` (default `docs/dependencies`).
    - `--ecosystems` (comma-separated subset of `go,js,python`). Defaults to auto-detect all supported ones.
    - `--include` / `--exclude` glob patterns for dependency names (optional).
    - `--rules` path for cleanup rules applied to fetched docs (reusing existing doc-rule format).
    - `--concurrency` limit (default derived from CPU count, capped at e.g., 8) for fetch requests.
    - `--llm-provider` / `--llm-endpoint` (optional) to enable heuristic lookups when repository metadata is missing.
  - Output is silent success with summary lines plus an optional `--format json` to emit a manifest describing written files.
- Configuration additions:
  - `config.yaml` gains a `doc_discover` block mirroring the flags above, so automation can run without arguments.
  - Boolean flags continue to rely on `internal/cli` boolean registrar to stay consistent with AGENTS.md requirements.

## Architecture Overview

```
cmd/ctx -> internal/cli (doc discover Cobra command)
            -> internal/discover (new package)
                 -> detectors (Go, JS, Python)
                 -> documentation source resolvers (manifest metadata, curated mappings, LLM fallback)
                 -> fetch orchestrator (wraps existing githubdoc.Fetcher + HTTP client)
                 -> persistence layer (writes Markdown via os.WriteFile, enforces layout)
            -> internal/output (renders summary or manifest)
```

- **New package** `internal/discover`:
  - `detector.Detector` interface: `Detect(ctx, rootPath) ([]Dependency, error)`.
  - `Dependency` smart constructor ensures name, version, ecosystem, and metadata (repository URL, homepage) are present.
  - `resolver.Resolver` pipeline:
    1. Manifest metadata (repository/homepage fields).
    2. Curated registry (map known package names → GitHub repo/doc roots).
    3. GitHub search fallback (rate-limited) for names without metadata.
    4. Optional LLM resolver (off by default) accepting prompts like “Find the canonical documentation for <package>”.
  - `fetcher` orchestrates downloads:
    - Reuse `internal/docs/githubdoc.Fetcher` for GitHub repositories.
    - Add `httpdoc.Fetcher` for non-GitHub doc sites (simple GET with Markdown/HTML conversion using `goquery` or `bluemonday` when needed).
  - `writer` generates files, ensures deterministic names (e.g., `docs/dependencies/go/github.com-spf13-viper.md`), and records a manifest.
  - `summary` collects successes/failures for CLI output and JSON manifest.

## Discovery Strategies by Ecosystem

### Go

- Parse `go.mod` with `golang.org/x/mod/modfile` (same helper already used by `RemoteDocumentationAttempt`) to list direct `require` entries.
- Optionally include `replace` and `// indirect` entries when `--include-indirect` flag is set (default false to stay focused on top-level dependencies).
- For each module:
  - Use module path to derive GitHub owner/repo (current logic already does this for remote documentation; we can reuse/extend).
  - Determine documentation roots: look for `docs/`, `doc/`, `README*.md`, and `website` directories.
  - If the repository is not on GitHub, fall back to `pkg.go.dev/<module>` scraping (HTML to Markdown) guarded by tests.

### JavaScript/TypeScript/CSS

- Parse `package.json` dependencies/devDependencies. Optionally respect `pnpm-lock.yaml`/`package-lock.json` to lock versions.
- Prioritize packages flagged as “runtime” (ignore devDependencies unless `--include-dev` is set).
- For CSS frameworks (e.g., Bootstrap, BeerCSS), treat them as npm packages so the same flow applies.
- Use `repository` field to map to GitHub, or `homepage` when repository missing.
- CDN references (e.g., `import` statements referencing `https://cdn.jsdelivr.net/...`) can be captured via a lightweight static analysis pass: scan `.html`, `.js`, `.css` files for CDN URLs and map hostnames to package names (jsDelivr watchers).
- Provide fallback mapping table (e.g., `bootstrap` -> `twbs/bootstrap`, `beercss` -> `beercss/beer.css`).

### Python

- Look for `pyproject.toml` (pep 621) and `requirements*.txt`. Use `pypi.org/p/<package>/json` to resolve the canonical project URL (HTTP GET, cached, respecting rate limits).
- Map to GitHub repository via `project_urls` or `info.home_page`.
- Limit to default extras; offer `--include-dev` to pull `dev-dependencies`.

## Documentation Retrieval Flow

1. Normalize dependency metadata into `Dependency` structs (`ecosystem`, `name`, `version`, `repositoryURL`, `docHint`).
2. For each dependency, run through resolver chain until a `DocumentationSource` is produced:
   - `RepositorySource` (owner, repo, reference, doc roots).
   - `WebsiteSource` (URL + DOM selectors to extract content).
   - `ManualSource` (inline string from curated registry for cases where docs are a single Markdown snippet).
3. Feed `RepositorySource` into existing `githubdoc.Fetcher` to reuse pagination, rule application, and token normalization. Extend `githubdoc` to accept custom reference or doc roots derived from dependency metadata.
4. Apply cleanup rules (same format as `ctx doc --rules`).
5. Persist Markdown file:
   - File header template with dependency metadata (name, version, source URL).
   - Body consists of concatenated documents separated by `---`.
   - Record manifest entry: `{name, ecosystem, version, source, relativePath, status}`.
6. Summaries:
   - CLI output prints table (# dependencies, fetched, skipped, failed).
   - JSON output (when requested) returns manifest for automation.

## Error Handling and Policy Alignment

- Smart constructors enforce required metadata; detectors return typed errors (e.g., `discover.ErrNoManifest`, `discover.ErrUnsupportedEcosystem`).
- Validation occurs at the detector layer (edge), keeping downstream fetchers free of redundant checks (POLICY.md compliance).
- Each failure path wraps errors with operation + subject (e.g., `fmt.Errorf(\"%w: resolve repository for %s\", ErrMissingRepository, dependency.Name)`).
- Provide `--fail-fast` (default false) to stop discovery on first error; otherwise continue collecting warnings.

## Testing Strategy

1. **Unit tests**
   - Detector tests using fixture manifests (`internal/discover/detector_go_test.go`, etc.) covering success, indirect filtering, malformed files.
   - Resolver tests verifying the mapping table, repository parsing, GitHub URL normalization, and fallback ordering.
   - Writer tests ensuring safe filenames and manifest accuracy (use `t.TempDir()`).
2. **Integration tests**
   - Spin up an HTTP test server that mimics GitHub/JSON endpoints; ensure `doc discover` fetches CSS/Go/Python docs end-to-end.
   - CLI test invoking `ctx doc discover` inside `tests/fixtures/project_with_go_and_js` verifying that Markdown files are created and summary output includes both ecosystems.
   - Failure-path tests (missing manifest, unauthorized GitHub, unreachable doc host) to assert warnings and exit codes.
3. **Regression tests**
   - Add golden-manifest snapshots to ensure deterministic ordering.
   - Add concurrency limit tests to confirm we respect the `--concurrency` flag (inspect worker count via instrumentation hooks).

## Implementation Phases (CT-101 roadmap)

1. **Scaffolding**
   - Introduce `internal/discover` package with domain types and detector interfaces.
   - Wire Cobra subcommand and configuration defaults.
2. **Ecosystem detectors**
   - Implement Go detector first (reusing `remote_attempt.go` logic).
   - Add JS detector (package.json parser) and CDN reference scanner.
   - Add Python detector using `pypi` metadata.
3. **Source resolvers**
   - Implement manifest-based resolver and curated registry.
   - Add GitHub search fallback (guarded by rate limiting).
   - Add LLM resolver behind feature flag; integrate with existing MCP client or a pluggable interface so offline builds skip it.
4. **Fetch + persistence**
   - Reuse/extend `githubdoc.Fetcher`, adding website fetcher where necessary.
   - Implement writer + manifest.
5. **Testing + docs**
   - Wire new integration tests under `tests/doc_discover`.
   - Update README/ARCHITECTURE with usage, configuration, and troubleshooting.

## Open Questions / Follow-ups

- Should indirect dependencies be documented by default, or only direct ones? (Plan assumes direct only; finalize during CT-101 implementation.)
- What is the retention story for generated docs? (Plan assumes overwriting existing files unless `--append` is set.)
- LLM-backed lookups require credentials; decide on provider (Anthropic/OpenAI/local) and env variable naming.
- Determine whether documentation fetching should respect existing `--doc` modes (`relevant` vs. `full`), or if discover always produces full bundles.
- Consider caching manifest results in `.ctx/doc_discover.json` to skip re-fetching unchanged dependencies.

This plan provides the concrete steps, architecture, and test strategy to implement `ctx doc discover` under CT-101.
