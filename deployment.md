# CTX Release And Deployment

## Resources

The repository declares four CLI resources and one GitHub Pages resource in `.mprlab/deploy/resources.yml`.
The release policy uses SemVer and the existing CTX version history.
The CLI entry point is the module root (`.`).
The `cmd/ctx` directory contains a library package.

Each CLI resource selects one platform and its compiler inputs.
All four resources build the same CTX source with CGO enabled.
Tree-sitter supplies the Python and JavaScript parsers.

| Platform | Resource | C compiler |
| --- | --- | --- |
| `linux/amd64` | `ctx-cli-linux-amd64` | `zig cc -target x86_64-linux-musl` |
| `darwin/amd64` | `ctx-cli-darwin-amd64` | `clang -arch x86_64` |
| `darwin/arm64` | `ctx-cli-darwin-arm64` | `clang -arch arm64` |
| `windows/amd64` | `ctx-cli-windows-amd64` | `zig cc -target x86_64-windows-gnu` |

The Linux resource selects `CGO_LDFLAGS=-static`.
Its binary contains the necessary C library code.
The macOS resources use the Apple SDK on the build host.
The Windows resource uses the Windows headers and libraries from Zig.

Gateway creates one `ctx-<os>-<arch>.tar.gz` archive for each platform.
Each archive contains `ctx` or `ctx.exe`.
The GitHub release uses the `tyemirov/ctx` repository.
Its release tag also identifies the Go module source for `go install`.

The documentation source is `docs/`.
Gateway publishes that source to `gh-pages` in `tyemirov/ctx`.
Deployment selects that branch as the GitHub Pages source.
The public URL is `https://ctx.mprlab.com/`.
Gateway creates and verifies `/.mprlab-release.json`.
Gateway also creates `CNAME` from the declared domain and an empty `.nojekyll` file.
Keep `CNAME`, `.nojekyll`, `.git`, and the verification marker outside the `docs/` source.
The initial Pages build reproduced the reserved source path failure before removal of `docs/CNAME`.

## Build Requirements

Use a macOS build host with Go, Apple Command Line Tools, and Zig on `PATH`.
Install Zig with `brew install zig`.
Use Gateway `v4.7.3` or later for the optional `build.environment` contract.

The application owns each CGO setting and compiler input.
Gateway supplies `GOOS`, `GOARCH`, and `MPRLAB_RELEASE_VERSION` for each build.
The manifest selects `CGO_ENABLED=1` and `CC` through each resource's `build.environment` mapping.
An inherited `CGO_ENABLED=0` does not replace these declared values.

Keep all four platforms and both language parsers.
Do not disable CGO to make a release build pass.
For an absent compiler or invalid compiler input, retain the native build error.

## Local Validation

Run `npm ci` to install the browser test dependencies.
Run `make ci` for Go lint, Go tests, and Puppeteer browser tests.
Run `make governance-check` to verify the managed guidance and Pages declaration.
Run `make check-release-build` to compile all four binaries from the manifest.

The release build test uses the `releasebuild` Go build tag.
This separate lane needs the macOS SDK and Zig.
The normal CI lane does not need these release compilers.
The release build test gives each artifact a temporary directory that Go removes after the test.
It verifies CGO, platform metadata, and the ELF, Mach-O, or PE architecture.
It also runs Python and JavaScript call chain commands through the native CTX artifact.
The initial test reproduced the missing CGO failure before the manifest correction.

## Operator Procedure

Complete the local checks before the production operation.
Commit and push the repository changes to its remote default branch (`master`).
Use a clean checkout whose `HEAD` equals `origin/master`.
Keep the installed Gateway and Governor commands on `PATH`.
Use `MPRLAB_GATEWAY_EXECUTABLE` to select an explicit installed Gateway command.
Use `MPRLAB_GOVERNOR_EXECUTABLE` to select an explicit Governor command.

The operator command is:

```bash
make release && make publish && make deploy
```

Each target checks governance before it calls the installed Gateway with the application Git root.
Release runs `make ci` and seals the declared artifacts.
Publication uses the sealed artifacts and records their immutable identities.
Deployment verifies the published binaries and the public documentation marker.

Gateway uses `$HOME/.config/mprlab-gateway` as its default operator root.
`MPRLAB_GATEWAY_OPERATOR_ROOT` selects another operator root.
GitHub operations use the active GitHub CLI credential.
These resources declare no Compose services or application private values.

## Evidence Boundaries

Local validation does not prove release, publication, or deployment success.
A successful cross-platform build proves artifact construction and format.
It does not prove runtime behavior on Linux, Windows, or a different macOS architecture.
Native parser tests prove the observed behavior on the test host.
Record each production result only after its lifecycle operation completes.

## Validation Evidence

The validation date is September 29, 2026.
The host uses macOS arm64, Go `1.27.1`, Apple Clang, and Zig `0.16.0`.
`make check-release-build` passed for all four platforms.
The Linux artifact has no dynamic loader dependency.
The native artifact passed both language parser checks.
Installed Gateway `v4.7.3` passed the resource validators and built all four archives in an isolated directory.
The installed Pages task passed after removal of the source `CNAME`.
Its archive contains the declared domain, empty `.nojekyll`, correct release marker, and unchanged `index.html`.
`make ci` passed Go lint, Go tests, and two Chromium browser tests at widths of 390 and 1280 pixels.
This validation did not run production lifecycle operations.
