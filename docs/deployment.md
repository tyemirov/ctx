# CTX Release And Deployment

## Resources

The repository declares one CLI resource and one GitHub Pages resource in `.mprlab/deploy/resources.yml`.
The release policy uses SemVer and the existing CTX version history.
The CLI entry point is the module root (`.`).
The `cmd/ctx` directory contains a library package.

The CLI resource declares these platforms:

- `linux/amd64`
- `darwin/amd64`
- `darwin/arm64`
- `windows/amd64`

Gateway creates one `ctx-<os>-<arch>.tar.gz` archive for each platform.
Each archive contains `ctx` or `ctx.exe`.
The GitHub release uses the `tyemirov/ctx` repository.
Its release tag also identifies the Go module source for `go install`.

The documentation source is `docs/`.
Gateway publishes that source to `gh-pages` in `tyemirov/ctx`.
Deployment selects that branch as the GitHub Pages source.
The public URL is `https://ctx.mprlab.com/`.
Gateway creates and verifies `/.mprlab-release.json`.

## Local Validation

Run `npm ci` to install the browser test dependencies.
Run `make ci` for Go lint, Go tests, and Puppeteer browser tests.
Run `make governance-check` to verify the managed guidance and Pages declaration.
Run `make check-release-build` to verify the installed Gateway binary build contract.

## Current Build Blocker

The lifecycle is **NOT READY** with Gateway `v4.7.2`.
CTX requires CGO for its Tree-sitter dependency.
The installed Gateway binary builder sets `CGO_ENABLED=0`.
Its `github_release_binary` contract has no CGO option or custom build adapter.
`make check-release-build` fails because the parser packages require CGO.

The affected runtime task is `deploy/ansible/playbooks/tasks/build-selected-release-github-binary.yml`.
The CTX manifest declares the supported resource shape.
The build boundary must support the CTX CGO requirement before release.
Keep all four platforms and the existing parser behavior when that change is selected.

## Operator Procedure

Resolve the build blocker before the production operation.
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
The former tag publication workflow is removed.

Gateway uses `$HOME/.config/mprlab-gateway` as its default operator root.
`MPRLAB_GATEWAY_OPERATOR_ROOT` selects another operator root.
GitHub operations use the active GitHub CLI credential.
These resources declare no Compose services or application private values.

## Evidence Boundaries

Local validation does not prove release, publication, or deployment success.
This preparation does not create a release receipt or a publication receipt.
It does not change the live Pages source or publish artifacts.
Record each production result only after its lifecycle operation completes.

## Preparation Evidence

The preparation date is September 29, 2026.
The source `HEAD` is `d8fe46f2bbc3d3d9d78448a89e6b0406318069d3`.
The preparation changes are local and uncommitted.
The installed Gateway source commit is `70ae14849f19eddc3876d27616ecce50047ba598`.
The runtime is `v4.7.2` on `darwin-arm64` with lifecycle contract `4`.

| Validation | Result |
| --- | --- |
| `make ci` | Passed Go lint, Go tests, and two Puppeteer tests. |
| Browser environments | Chromium at widths of 390 and 1280 pixels with Node 24. |
| `make governance-check` | Passed with no drift. |
| Installed resource and graph validators | Passed for both resources with no remote changes. |
| Make target declarations | Passed the dry run for all three operations and the executable override. |
| `make check-release-build` | Failed at the Tree-sitter CGO boundary. |
| `app-plan-release` | Rejected the uncommitted checkout before manifest capture. |
| Changed prose | No new mechanical findings. The language review covers the changed text. |
| Existing prose | 25 mechanical findings remain in unchanged README and architecture text. |

There is no selected release digest or sealed receipt from this preparation.
The production state revision, owner generation, fences, and observations were not inspected.
The build failure prevents a ready production handoff.
