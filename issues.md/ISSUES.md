# ISSUES 
**Append-only section-based log. Each section (Features, Improvements, BugFixes, Maintenance, Planning) is append-only**

Entries record newly discovered requests or changes, with their outcomes. No instructive content lives here. Read @NOTES.md for the process to follow when fixing issues.

Read @AGENTS.md, @AGENTS.GIT.md, @AGENTS.DOCKER.md, AGENTS.GO.md, @AGENTS.FRONTEND.md, @POLICY.md, PLANNING.md, @NOTES.md, and @ISSUES.md under issues.md/. Read @ARCHITECTURE.md, @README.md, @PRD.md. Start working on open issues. Work autonomously and stack up PRs. Prioritize bugfixes.

Each issue is formatted as `- [ ] [CT-<number>]`. When resolved it becomes `- [x] [CT-<number>]`

## Features (104–199)

- [ ] [CT-103] Add an LLM support to format the documents as on of the lasts steps in `doc` command.
    1. Prepare a prompt that asks to 
        a) deduplicate the data and identify and remove repeated blocks of text unrelated to the documentation itself, such as footers, headers, privacy notes etc
        b) Keep the documentation itself without changes and do not alter it, quoting it back verabitium
    2. Use the LLM integration available in @tools/gix package to query LLMs
    3. Consider that the documents can be lengthy and we shall not alter them, but remove fluff only

- [x] [CT-104] Add a website documenting all of the benefits the ctx utility has. The web site shall be served from github so follow the convention for folders/file placement.
1. Use docs/index.html so that GitHub can easily find it
2. Prepare the content as a technical sales person appealing to the end user
3. Use mpr-ui library and leverage footer from it @tools/mpr-ui/docs/custom-elements.md, @tools/mpr-ui/README.md
    - Added a marketing-focused `docs/index.html` that introduces ctx, highlights its core commands and workflows, and integrates the `mpr-ui` footer via the CDN bundle so GitHub Pages can serve a polished, static documentation site.

## Improvements (213–299)

## BugFixes (300–399)

- [x] [CT-300] `ctx doc discover` does not process JS even when package.json is right at the root of the repo. Ensure we have tests for such cases
```
17:02:06 tyemirov@computercat:~/Development/mpr-ui [improvement/MU-108-custom-element-docs] $ ll package*
-rw-rw-r-- 1 tyemirov tyemirov 482 Nov  7 16:59 package.json
-rw-rw-r-- 1 tyemirov tyemirov 40K Nov  7 16:28 package-lock.json
17:02:13 tyemirov@computercat:~/Development/mpr-ui [improvement/MU-108-custom-element-docs] $ ctx doc discover
Dependencies processed: 0 (written: 0, skipped: 0, failed: 0)
```
    - Automatically fall back to JavaScript `devDependencies` when runtime dependencies are absent and added regression coverage for the fallback plus the explicit `--include-dev` path so dev-only repos surface documentation.
- [x] [CT-301] `go run ./...` shall work and invoke our cmd/ctx/main.go.  make sure that both scenarios work without errors:
1. go run ./...
```
12:11:44 tyemirov@Vadyms-MacBook-Pro:~/Development/tyemirov/ctx - [bugfix/CT-300-js-discover-root] $ go run ./...
go: pattern ./... matches multiple packages:
        github.com/tyemirov/ctx
        github.com/tyemirov/ctx/cmd/ctx
```
2. Retain an ability to install, e.g. `go install github.com/tyemirov/ctx@latest` must work
    - Routed the root `main.go` through the shared `cmd/ctx` bootstrap, removed the duplicate main package, and added an integration test that runs `go run ./... --version` so the installer path and `go run` both succeed.

## Maintenance (403–499)

## Planning
do not work on the issues below, not ready
- JS discovery improvements:
  - Allow recursive manifest detection (e.g., `**/package.json` outside `node_modules/`) so repos with nested web apps get documented in one pass. Needs glob/ignore support, caching, and CLI flags (`--package-json-glob`?) to keep large workspaces manageable. Must dedupe dependencies discovered in multiple subtrees and keep output directories stable.
  - Optional LLM/search-backed hinting phase for “no docs found” cases. Prompt model with README text + directory listing to suggest candidate doc paths or external doc URLs (validated before fetch). Requires provider flag (`--llm-provider`), caching responses per dependency, domain allowlist for external fetches, and clear user-facing opt-in because of latency/cost.
