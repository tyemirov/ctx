# I001 Shared UI Migration

I001 prepares the CTX documentation page for mpr-ui I009.
The application source starts at `0f7beb749860091637e417ec8d17146da489c9b5`.
The shared candidate is `768f25936497c5aabd426197d21c2100b6e5d9a1`.
Its SHA-256 values are defined in `tests/shared-ui-candidate.mjs`.

## Release Unit

Publish `docs/index.html` with the current footer menu.
The page retains literal `@latest` shared JavaScript and CSS URLs.
The Resources menu preserves GitHub, Docs, Changelog, and Community links.
The MIT license link and modal retain their content.
The page has no authentication config or theme control.

GitHub reports the production domain as `ctx.mprlab.com`.
The current Pages source is `master` with the `/docs` directory.
Publication preparation must include the required `gh-pages` resource and release identity.
The public release marker currently returns HTTP 404.

## Validation

Both real-page regressions failed before the footer change because the current shared candidate did not render the previous links input.
The focused checks then passed at 390 and 1280 pixels.
They verify menu links, horizontal bounds, keyboard dismissal, focus, license content, and page reload.
The tests retain the real page and shared assets, with controlled telemetry responses.
Each shared asset has a verified SHA-256 value.
The page keeps its existing theme behavior.

Use `npm ci` to install the browser test dependencies and Chromium.
Use `make test-browser` for the focused checks.
Use `make ci` for formatting, Go vet, Go tests, and browser checks.
The documentation workflow runs the same browser target for page, test, dependency, and workflow changes.
Final B069 local `make ci` passed formatting, Go vet, all Go tests, and both browser checks.
The log is `/tmp/ctx-i001-b069-ci.log`.
Local CI includes the separate telemetry and governance changes in the primary checkout.
Hosted CI qualifies the committed migration source separately.
Hosted browser CI passed at `d38791cea7a2d2b892f7f785274503dd62bedd75`.
The [hosted run](https://github.com/tyemirov/ctx/actions/runs/34302289256) uses the same browser target.

## Publication And Acceptance

The [public asset record](mpr-ui/public-assets-2026-09-09.json) contains four observations from one network location.
The page and both shared assets return HTTP 200. The release marker returns HTTP 404.
The shared assets permit a seven-day browser cache and a twelve-hour shared cache.
These observations do not establish the coordinated cache transition.

1. Complete the mpr-ui I009 application preparation and final candidate qualification.
2. Prepare the Pages publication resource, release identity, and maintenance artifact.
3. Let the user select the production window and run the publication sequence.
4. Verify the public page and both shared asset digests after the cache transition.
5. Verify resource links, keyboard controls, and the MIT license at mobile and desktop widths.
6. Keep I001 blocked until publication preparation, cache transition, and public acceptance pass.
