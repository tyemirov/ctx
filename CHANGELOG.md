# Changelog

## [v0.5.0] - 2026-07-08

### Features ✨

- Introduced the `bundle` CLI command for extracting goal-oriented execution contexts.
- Added frontend context building for bundles, providing structured context output.
- Implemented depth-limited web documentation extraction via an HTML crawler for non-GitHub URLs in `ctx doc`.

### Improvements ⚙️

- Updated Google Analytics integration for the web frontend.
- Enhanced documentation and architecture files to reflect new entry points and internal structure.
- Improved handling of JavaScript dev dependencies as fallback when runtime dependencies are missing.

### Bug Fixes 🐛

- Resolved duplicate `package main` by consolidating entry points and wiring `main.go` through the CLI bootstrap.
- Fixed issues with relative links in site documentation pages.

### Testing 🧪

- Added targeted tests for context bundle exclusion rules (e.g., budget, secret files).
- Verified bundle command integration with frontend context and exclusion logic.
- Expanded test coverage for webdoc extraction and MCP command execution.

### Docs 📚

- Documented the forward-only contract discipline in `AGENTS.md`.
- Revised and expanded `ARCHITECTURE.md` with updated package layouts and runtime overviews.
- Refined `README.md` for user workflows and added a new `CNAME` for site deployment.

## [v0.4.0]

### Features ✨

* Added `ctx doc discover`, a dependency-aware documentation generator that scans Go/npm/Python manifests, fetches upstream docs via the shared GitHub client, and writes Markdown bundles plus JSON manifests, complete with new configuration knobs (`doc_discover`), clipboard integration, and test-backed registry/GitHub stubs (CT-101).
* `ctx doc` now accepts either GitHub coordinates or arbitrary HTTPS documentation URLs, automatically switching to the depth-limited web crawler (with a new `--web-depth` flag) instead of relying on a separate subcommand. External captures keep clipboard support and MCP parity (CT-102, CT-212).

### Docs 📚

* Added `docs/doc_discover_plan.md`, capturing the CLI contract, architecture, dependency detection heuristics, and phased rollout strategy for the upcoming `ctx doc discover` feature (CT-100).

### Improvements ⚙️

* Switched the default `ctx doc discover` output directory from `doc/dependencies` to `docs/dependencies` so generated bundles align with standard documentation layouts (CT-211).
* Updated the JavaScript detector to fall back to `devDependencies` when a project has no runtime dependencies, ensuring `ctx doc discover` still surfaces documentation for repositories like `mpr-ui` without extra flags (CT-300).

### Bug Fixes 🐞

* Removed the duplicate `package main` and wired the module-root `main.go` through the shared `cmd/ctx` bootstrap so `go run ./...` now executes the CLI while `go install github.com/tyemirov/ctx@latest` continues to install the binary (CT-301).

## [v0.3.1]

### Docs 📚

* Refocused the README around user workflows and split deep technical guidance into `ARCHITECTURE.md`.
* Expanded `ARCHITECTURE.md` with package layout, data flow, and subsystem details to match the current implementation. 
* Broke the refactoring roadmap into tracked backlog issues (CT-205–CT-210) covering CLI modularisation, configuration invariants, streaming resilience, and safety-net tests.

### Improvements ⚙️

* Rebuilt CLI command orchestration around shared descriptors, context-aware execution, and Cobra-managed writers, with expanded coverage for clipboard and format validation.
* Injected an explicit call chain service so CLI and MCP flows configure analyzers without hidden globals, with tests covering service wiring.
* Hardened configuration copy invariants with typed clipboard settings and expanded coverage for merge precedence.
* Added documentation option value objects, centralized GitHub token resolution, and tokenizer helper sentinel errors so remote docs/token flows emit contextual failures backed by new CLI/tokenizer tests.
* Extended safety nets with a Cobra-driven `--copy-only` test, new MCP negative HTTP scenarios, and fresh utility coverage for size/time/mime helpers.

## [v0.3.0]

### Improvements ⚙️

* Switched the CLI default output to TOON and added dedicated renderers for tree, content, and call chain results while keeping raw/json/xml options. (improvement/CT-04-toon-default)

## [v0.2.3] - 2025-10-26

### Bug Fixes 🐞

* Respected parent `.gitignore` and `.ignore` directives when scanning subdirectories so tree/content output no longer leaks excluded folders. (bugfix/CT-01-ignore-parent-folders)

### Improvements ⚙️

* Added `--copy-only` (`--co`) to mirror clipboard output without printing to stdout, plus configuration support via `copy_only`. (improvement/CT-02-copy-only)
* Introduced the short `--c` alias for `--copy`, keeping existing behaviour while offering a quicker toggle for clipboard mirroring. (improvement/CT-03-copy-alias)

## [v0.2.2] - 2025-10-25

### Highlights

* **Repository owner rename and module path alignment.** The project now lives at `github.com/tyemirov/ctx`, and every import, configuration default, and installer reference has been migrated so downstream builds keep working after the move.

### Improvements ⚙️

* Raised the minimum Go toolchain to 1.25, matching the repository relocation and ensuring `go install github.com/tyemirov/ctx@latest` succeeds with the updated module path.

### Docs 📚

* Updated README badges, installation instructions, and call chain examples to point to the new `tyemirov` GitHub namespace.

## [v0.2.1] - 2025-10-19

### Highlights

* **Consistent CLI booleans and clipboard flag rename.** All commands now share a single boolean flag parser that understands true/false, yes/no, on/off, and 1/0
literals, and the former `--clipboard` flag has been renamed to `--copy` with configuration defaults preserved.

### Improvements ⚙️

* Replaced every direct `BoolVar` registration with the shared registrar, eliminating bespoke parsers and ensuring `--flag no` works everywhere. (improvement/CT-22-
bool-flags)
* Stopped normalizing boolean flags after the `--` sentinel so dash-prefixed filenames (e.g., `ctx tree -- --summary`) are handled correctly. (improvement/CT-22-
bool-flags)
* Updated configuration loading to honor both legacy `clipboard` and new `copy` keys, retaining overrides from user config. (maintenance/CT-21-copy-flag)

### Docs 📚

* Documented the expanded boolean syntax and the new `--copy` flag in the README, along with guidance for enabling automatic copy via config.

### Testing ✅

* Added unit coverage for the boolean flag registrar, including argument normalization around the `--` terminator and non-boolean trailing values.

## [v0.2.0] - 2025-10-18

### Highlights

* **GitHub-backed documentation retrieval via `ctx doc`.** A dedicated command now walks repository directories through the GitHub Contents API, applies rule-driven cleanup, and renders curated documentation in raw, JSON, or XML formats with clipboard support. (feature/CT-16-github-docs)
* **Human-readable CLI diagnostics.** Fatal errors surface as plain text alongside richer flag guidance, keeping workflows approachable for operators and MCP clients alike. (bugfix/CT-doc-help)

### Features ✨

* Added `internal/docs/githubdoc` with recursive fetchers, rule set loaders, and sorting to deliver deterministic documentation bundles for remote repositories. (feature/CT-16-github-docs)
* Introduced the `doc`/`d` Cobra command with flags for owner, repo, ref, rules, documentation mode, and clipboard usage, seamlessly reusing configuration defaults. (feature/CT-16-github-docs)

### Improvements ⚙️

* Removed the deprecated static documentation rule scaffolding in favor of runtime rule evaluation sourced from local files. (feature/CT-16-github-docs)
* Redesigned `ctx doc` error handling and help text to spell out unified `--path` coordinates and clipboard options. (bugfix/CT-doc-help)
* Switched zap to console encoding without metadata noise so `application execution failed` lines print as a single human-readable sentence. (bugfix/CT-doc-help)
* Consolidated doc command coordinates behind a single `--path` flag that accepts `owner/repo[/path]` specs or GitHub URLs while preserving explicit `--ref` overrides. (maintenance/CT-19-doc-path)

### Docs 📚

* Documented doc command parameters in README, outlining required coordinates, optional flags, and clipboard behavior. (bugfix/CT-doc-help)

### Testing ✅

* Added integration tests that spin up a GitHub mock server and verify `ctx doc` across full and relevant modes for jspreadsheet, marked.js, and beer.css fixtures. (feature/CT-16-github-docs)

### Maintenance 🛠️

* Removed `PLAN.md` from version control and added a guard test that fails when the file becomes tracked again. (maintenance/CT-20-plan-guard)

## [v0.1.2] - 2025-10-18

### Highlights

* **CGO-enabled builds across CI and release targets.** Test and release workflows now compile with `CGO_ENABLED=1`, ensuring Python and JavaScript call-chain analyzers are always included. Windows releases automatically install a MinGW toolchain so tree-sitter bindings build deterministically. (maintenance/CT-15-enable-cgo)

### Improvements ⚙️

* Switched the test workflow to run with CGO enabled, preventing accidental regressions in language analyzers. (maintenance/CT-15-enable-cgo)
* Reworked the release pipeline to build per-platform artifacts on native runners, upload them as artifacts, generate checksums, and publish consistently. (maintenance/CT-15-enable-cgo)
* Hardened release note extraction so publishing succeeds even when the changelog entry is missing. (maintenance/CT-15-enable-cgo)

### Maintenance 🛠️

* Installed the MinGW toolchain on Windows release runners, guaranteeing CGO builds succeed without manual setup. (maintenance/CT-15-enable-cgo)

## [v0.1.1] - 2025-10-17

### Highlights

* **Richer MCP contract with advertised workspace metadata.** The MCP server now exposes `/environment` and includes the root directory in `/capabilities`, guiding agents to send absolute paths. (ba36898)
* **Comprehensive MCP end-to-end coverage.** A new integration test starts `ctx --mcp`, queries capabilities, and exercises tree/content commands over HTTP against a seeded project. (ecdacec)

### Features ✨

* Documented workflow for registering `ctx` with Claude Desktop and Codex, including JSON-only semantics and environment probing. (69165c4, daec63f)

### Improvements ⚙️

* Expanded capability descriptions to list supported flags and JSON-only behavior, making the MCP surface self-descriptive for agents. (daec63f)

### Docs 📚

* Added instructions for resolving paths via `/environment` and registering MCP servers with popular clients. (69165c4)

### Testing ✅

* Added an automated MCP end-to-end test that launches the server, inspects capabilities, and runs tree/content commands using absolute paths. (ecdacec)

## [v0.1.0] - 2025-10-14

### Highlights

* **Run ctx as an MCP server with JSON endpoints.** The CLI now exposes tree, content, and callchain commands over HTTP with structured capability metadata and an environment probe, ready for orchestrators. (615e90e, 245efa5, 9a6a099, ba36898, daec63f)
* **Cross-language callchain analysis with documentation harvesting.** Go, Python, and JavaScript analyzers collaborate with a new collector to surface external docs alongside call graphs. (cc2fc6f, 2d6c9c5, 47d0498)

### Features ✨

* Introduced the `--mcp` flag to launch an MCP server that accepts JSON payloads, dispatches built-in commands, and returns normalized responses with warning streams. (615e90e, 245efa5, 9a6a099)
* Added configuration bootstrap and discovery: `--init` scaffolds local or global config, `--force` handles overwrites, and `--config` loads per-command defaults via Viper. (5e9f0d9, f95f300)
* Enabled multi-language documentation extraction so content listings can enrich files with Go, Python, and JavaScript API references. (2d6c9c5)
* Registered language-specific callchain analyzers and a dispatcher to resolve symbols across Go, Python, and JavaScript projects. (cc2fc6f)

### Improvements ⚙️

* Unified tree and content streaming through a shared event dispatcher to keep raw, JSON, and XML outputs synchronized. (f95f300)
* Hardened MCP command handling by enforcing JSON formats and per-command defaults for inputs and warnings. (9a6a099, 245efa5)
* Added a clipboard service abstraction so `--clipboard` copies output without leaking platform specifics. (f95f300)
* Fell back to rune-based counting when tokenizer models are unavailable, while reporting the requested model name. (2d6c9c5)

### Fixes 🐛

* Blocked nested call capture in Python and JavaScript analyzers to remove duplicate edges from call graphs. (47d0498)

### Docs 📚

* Expanded MCP documentation with local server workflows, Claude and Codex integration notes, and JSON contract details. (69165c4, 2530046, c620c45)
* Captured future roadmap and portability notes for CT-01 and CT-02 scenarios. (2d59656, cc7a681, 76fef18)
* Refined README formatting and release linkage for improved navigation. (527ad95, b0272f8, 446e414)

### Testing ✅

* Added an end-to-end MCP test suite that provisions sample projects and verifies command execution over HTTP. (ecdacec)

## [v0.0.15] - 2025-10-06

### Highlights

* **Real-time, schema-valid streaming across all formats.** Tree and content commands now share a single streaming
  pipeline that emits per-directory and per-file events, producing identical results for raw/JSON/XML while displaying
  output incrementally. (3f80e9d, e858f67, 5d8895b, da3f01a)
* **Resilient tokenizer integration.** Python helpers are autodetected via `uv`, sentencepiece models download on demand,
  and Claude model aliases map automatically so token counting “just works” without manual setup. (6af6a24, b8a6330,
  a7b3354, 979e156, 909039b)

### Features ✨

* Added automatic `uv` invocation for helper scripts, including on-demand sentencepiece downloads for Llama models and
  self-contained Anthropic helpers. (6af6a24, b8a6330, a7b3354, 979e156)
* Tokenizer metadata is now surfaced in both tree and content summaries, with default summaries enabled for every run.
  (d4e42c5, 46b982f, ea3098e)

### Improvements ⚙️

* Unified tree/content streaming walker captures file contents, streams chunks, and keeps JSON/XML renderers in sync
  while maintaining pretty-printed output. (3f80e9d, 5d8895b, da3f01a)
* CLI displays streaming progress while helpers count tokens and ensures trailing newlines so prompts resume cleanly.
  (509805a, d02f35c, e858f67)
* Fixed double counting in summaries and guaranteed `done` events even when traversal encounters errors. (9c511ab)

### Fixes 🐛

* Removed spurious helper bootstrap output and normalized Anthropic payloads, preventing API failures. (7f09f1a,
  b8f3737, 2962cf9)
* Dropped stale llama warning suppression and ensured Anthropic helpers require a valid API key with helpful messaging.
  (e96e07c, 9c73429)

### Docs 📚

* Documented tokenizer model support, uv requirements, and the need to quote shell wildcards when excluding files.
  (663c7f9, edce64b, dee8da2)

### Maintenance 🛠️

* Updated the toolchain to the latest Go release. (e15aa61)

### Tests ✅

* Expanded streaming coverage, including helper-driven tokenization and end-to-end CLI fixtures for raw/JSON/XML output.
  (748785c, 4b84cbb, e14ea9c)

**Upgrade notes:** Ensure `uv` is on `PATH`; the CLI will fetch helper dependencies and models automatically. Provide an
`ANTHROPIC_API_KEY` when requesting Claude token counts.

## [v0.0.14] - 2025-09-06

### Highlights

* **Consistent logging with Zap.** Integrated `zap` structured logging across the codebase for clearer and more reliable
  log output. (#86 b5ac3d5, #88 ddeb310, #89 7579eef)
* **Error and constant refactoring.** Centralized error messages and shared constants for better maintainability and
  consistency. (#76 2684689, #79 80cdb67, #81 dd4e06a, #87 2188b4d)
* **Tree builder improvements.** Introduced a `TreeBuilder` struct, removed duplication, and optimized MIME type
  detection. (#74 9b48d0c, #84 e77abaa, aec289e, 588d81a)

### Features ✨

* Added `zap` logging integration with structured fields and standardized log output. (#86 b5ac3d5, #88 ddeb310, #89
  7579eef)

### Improvements ⚙️

* Centralized and reused error message constants across multiple packages. (#76 2684689, #81 dd4e06a, #87 2188b4d,
  fec2804)
* Introduced `TreeBuilder` abstraction and improved binary/MIME type detection logic. (#74 9b48d0c, #84 e77abaa,
  aec289e, 588d81a, b551cee)
* Refactored variable and parameter names for clarity (e.g., `lastDotIndex`, descriptive sort params, call chain
  vars). (#78 a01f6cb, #79 80cdb67, #83 b4e47fa)
* Shared constants for documentation entry prefixes and error log formatting. (#68 d4caa16, #70 8bdd50b, #71 991ec54,
  c2247ba, 790c401)
* Improved output helpers and sorting of documentation entries. (44861a7, afc08e3, 5bd553f)
* More robust error handling for working directory resolution in CLI. (#69 a547738, dd1128a)

### Docs 📚

* Expanded CLI help text and clarified `--doc` flag behavior. (#64 aaf807b, #65 23929a1, 391b01a, 0a73f97)
* Added Quick Start guide and documented default JSON output format. (#66 ca4831d, #67 124f2af, 159e62c, 81ebfe2)
* Documented XML output format. (44da621)

### Maintenance 🛠️

* Removed unused constants (e.g., `xmlCallchainName`, `gitDirName`). (#72 50ce9ae, #87 2188b4d)
* Fixed branch naming inconsistency (`master` as default). (914a72b)

**Upgrade notes:** No breaking changes. Logging is now structured with Zap; existing CLI flags and outputs remain
compatible.

## [v0.0.13] - 2025-09-05

### Highlights

* **Repository structure improvements.** Exposed a root-level `main.go` for easier CLI builds, with shared Cobra setup
  for consistency. (#60 2686dae, c63e0af)
* **More flexible exclusion handling.** Multiple adhoc exclusion patterns are now supported in a single run. (#61
  fec03ce, 6aa9093)
* **Streamlined release flow.** Documented release steps for contributors and automated CI to run only on Go file
  changes. (#62 822d428, d9c9cd5, #63 7900db2, 30c2263)

### Features ✨

* Expose root-level `main.go` entry point and shared CLI configuration. (#60 2686dae, c63e0af)

### Improvements ⚙️

* Allow multiple exclusion patterns via `-e/--exclude`. (#61 fec03ce, 6aa9093)

### Docs 📚

* Add release preparation and publishing instructions. (#62 822d428, d9c9cd5)

### CI & Maintenance

* Run tests only when Go files change. (#63 7900db2, 30c2263)

**Upgrade notes:** No breaking changes. Builds, flags, and outputs remain compatible.

## [v0.0.12] - 2025-09-04

### Highlights

* **End-to-end MIME detection.** All files now get a detected MIME type; `tree` nodes include `mime` for file entries. (
  #57 e7ad015, #56 c587e9c, b58837c, b628a44)
* **CLI UX overhaul.** Custom usage template, improved help layout, and command aliases shown in help; help displays
  when no command is provided. (#55 a99dc40, a93b618, #47 aa8f792, #45 3de79fb, 60f0d10)
* **Ignore system hardening.** Refined path-based ignore logic, centralized ignore constants, clarified docs, and added
  `[binary]` section support in `.ignore`. (#54 14b5190, #51 ff3756a, #52 96c691c, #32 a6f07cb, 31d4603)
* **Stricter output validation.** Centralized output-format checks. (#35 e2b78f7, c83a8b4)
* **XML output format.** Added `xml` to supported output formats. (#20 da50618, 8f72db5)
* **Much broader test coverage.** New and expanded tests across `content/ctx`, ignore handling (including nested
  `.gitignore`), MIME detection, utilities, and output. (#53 1f7585b, #50 048a866, #48 0267170, #30 52fb374, #27
  7a98f75, 9203bbb, a117010, 6fe58e4, 73b7b01)

### Features ✨

* Detect MIME type for all files and surface MIME in `tree` file nodes. (#57 e7ad015, #56 c587e9c, b58837c, b628a44)
* Add `xml` to list of output formats. (#20 da50618, 8f72db5)

### Improvements ⚙️

* Support repeatable exclusion flag with glob pattern matching for tree and content commands.
* Improve help output and adopt a custom usage template. (#55 a99dc40, a93b618, #47 aa8f792)
* Show command aliases in help (Cobra help template override). (#45 3de79fb, c311f97)
* Refine path-based ignore handling and centralize ignore constants/messages. (#51 ff3756a, #41 ac2da74, 22bb964,
  a64d9cd, 59c829b)
* Extract common path flags and flag-registration helpers for consistency. (#44 cab1575, 101c207)
* Validate output formats via a single function. (#35 e2b78f7, c83a8b4)

### Fixes 🐛

* Show help when no command is provided (CLI regression fix). (#31 5493242, 60f0d10)

### Docs 📚

* Clarify ignore pattern handling and document related utilities. (#52 96c691c, 31d4603)
* Document call-chain depth flag and related behavior. (#29 ed3382f, f29acd5)
* Add GoDoc comment for `findGitDirectory`. (#34 2004ec2, ee5d754)
* Update README with new features; remove manual changelog fragment. (#25 541caeb, 47834ad)

### Tests ✅

* Add content command test for nested `.gitignore` and broader `ShouldIgnore` coverage. (#53 1f7585b, f6d517c, 1abe325)
* Add fixtures (incl. Google Sheets add-on) and extend `ctx_test.go`. (#50 048a866, a117010)
* Add tests for internal utils and output functions; call-chain depth tests; binary MIME tests. (#48 0267170, 6adc791,
  9203bbb, 73b7b01)

### Refactors 🧹

* Use descriptive loop/variable names across codebase. (#40 5d44468, #38 066afa0, f52116b, e73466f)
* Remove unused constants and legacy/binary patterns from tree handling. (#39 c0cf7cd, 24f6ce6, 77a27f0)
* Remove legacy `show-binary-content` directive (now superseded by `[binary]` in `.ignore`). (446504c)

### CI & Maintenance

* Run Go tests on pull requests. (#7 d8df33c, c0c0c95)
* License update. (c31641f)

**Upgrade notes:** No breaking changes expected in this release. Existing CLI flags and outputs remain compatible; the
new XML format and richer help/alias displays are additive.

## [v0.0.11] - 2025-04-20

### Features ✨

1. **Binary Section in `.ignore`**
    - `.ignore` files now support a `[binary]` section listing patterns whose binary contents are base64-encoded in
      output.
    - The legacy `show-binary-content:` directive has been removed.

## [v0.0.10] - 2025-04-19

### Features ✨

1. **Documentation Extraction (`--doc` flag)**
    - Added the `--doc` boolean flag to the `content` (`c`) and `callchain` (`cc`) commands.
    - When enabled, ctx collects and embeds documentation for imported third‑party packages and referenced functions.
    - Documentation is available in both *raw* and *json* output formats.

### Improvements ⚙️

1. **Tests Package Renaming**
    - All integration tests are now consolidated under the `tests` package to simplify discovery.
2. **Table‑Driven Tests**
    - Introduced table‑based patterns for new test cases to improve readability and maintenance.

### Internal

1. **README Update**
    - Added details about the new `--doc` flag and updated flag tables and examples.

---

## [v0.0.9] - 2025-04-15

### What's New 🎉

1. JSON is a default output format

## [v0.0.8] - 2025-04-15

### What's New 🎉

1. Renamed `content` to `ctx`

## [v0.0.7] - 2025-04-13

### Features ✨

1. **Call Chain Analysis Command (`callchain`, `cc`):**
    - Added a new command `callchain` (alias `cc`) to analyze the call graph for a specified function within the
      repository.
    - Usage: `content callchain <function_name> [--format <raw|json>]`
    - The `<function_name>` argument should be a fully qualified name (e.g., `github.com/org/repo/pkg.MyFunction`) or a
      unique suffix.
    - **Raw Output (`--format raw` or default):** Displays the target function, its direct callers, and its direct
      callees in a human-readable format, followed by the source code of these functions.
    - **JSON Output (`--format json`):** Outputs a JSON object containing the `targetFunction`, arrays of `callers` and
      `callees` (fully qualified names), and a `functions` map where keys are function names and values are their source
      code strings.

## [v0.0.6] - 2025-04-08

### Features ✨

1. **Version Flag:** Added a `--version` flag to display the application version.

### Bug Fixes 🐛

1. **Version Verbiage:** Corrected the wording used when displaying the version information.

## [v0.0.5] - 2025-03-29

### Features ✨

1. **JSON Output Format:**
    - Added a `--format <type>` flag.
    - Supported types are `raw` (default) and `json`.
    - `--format json` outputs results in a structured JSON format suitable for machine processing (e.g., by AI tools).
    - `content content --format json`: Outputs an array of objects, each with `path`, `type` ("file"), and `content` for
      successfully read files.
    - `content tree --format json`: Outputs an array representing input paths. Files have `path` and `type`. Directories
      have `path`, `type`, and nested `children`.
    - The default `raw` format preserves the original human-readable text output.

## [v0.0.3] - 2025-03-29

### Features ✨

1. **Multi-Directory Support:**
   The tool now accepts multiple directory paths as positional arguments.
   `content <command> [dir1] [dir2] ... [flags]`
    - `.ignore` and `.gitignore` files are loaded relative to *each* specified directory.
    - The `-e`/`--e` exclusion flag applies to direct children within *any* of the specified directories.
    - Output for `tree` shows separate trees for each directory.
    - Output for `content` concatenates file contents from all specified directories sequentially.
    - Duplicate input directories (after resolving paths) are processed only once.
    - If no directories are specified, it defaults to the current directory (`.`).

2. **Mixed File/Directory Input:**
   The tool now accepts both file and directory paths as positional arguments.
   `content <command> [path1] [path2] ... [flags]`
    - For the `content` command:
        - Explicitly listed files have their content printed directly.
        - Directories are traversed recursively, printing content of non-ignored files within them.
    - For the `tree` command:
        - Directories are traversed, displaying their structure.
        - Explicitly listed files are shown with a `[File]` marker and their absolute path.
    - Ignore rules (`.ignore`, `.gitignore`, `-e`) only apply during the traversal of *directory* arguments. Explicitly
      listed files are *never* filtered by ignore rules.

## [v0.0.2] - 2025-03-23

### What's New 🎉

1. **Using `.ignore` Instead of `.ignore`:**
   The tool now looks for a file named **.ignore** for exclusion patterns.

2. **Processing `.gitignore` by Default:**
   The tool loads **.gitignore** by default if present. Use the `--no-gitignore` flag to disable this.

3. **Folder Exclusion Flags:**
   Both **-e** and **--e** flags now work for specifying an exclusion folder. The folder is excluded only when it
   appears as a direct child of the working directory.

4. **Disabling Ignore File Logic:**
    - **--no-gitignore:** Prevents the tool from reading the **.gitignore** file.
    - **--no-ignore:** Prevents the tool from reading the **.ignore** file.

5. **Command Abbreviations:**
   Short forms **t** for **tree** and **c** for **content** are now supported.
