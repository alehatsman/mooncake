# mooncake CI image

`mgitci.yml` runs `mooncake task ci` (the full pre-push gate) inside a container
that moongit spins up per job. This directory builds that image,
`mooncake-ci:latest`.

## Build

The image derives from `moongit-ci-dev:latest` (Go + the static-analysis
toolchain + a static `mooncake` + git + python3) and adds `gomarkdoc`, which the
docs-regen step (`[6/11]`) shells out to. Build that base first (see
`moongit/ci/README.md`), then from the mooncake repo root:

```sh
docker build -t mooncake-ci:latest ci/
```

The final `gomarkdoc --version` step fails the build early if the install
didn't land on PATH.

## Why this image

`moongit-ci-dev:latest` already carries everything `mooncake task ci` needs
*except* `gomarkdoc`. Rather than re-bake the whole Go/lint toolchain, this
image is a thin derive-and-add. `gomarkdoc` is pinned to `v0.4.1` to match
`tasks.yml`'s `docs-tools-install`, so CI and the local docs pipeline render
byte-identical output.

The mkdocs build step (`[7/11]`) self-skips when `pipenv`/`mkdocs` are absent,
so they are intentionally left out. If you want CI to build the docs site too,
derive a further image that installs them and point `mgitci.yml`'s `image:` at
it.

## Module fetch

`mooncake task ci` pulls the shared `go-quality` module from
`github.com/alehatsman/go-quality` over https (pinned in `tasks.yml`). Nothing
else is wired for it: the job needs no host trust, no `--add-host`, and no route
back to the moongit host, so a throwaway container with an empty module cache
resolves it on its own.

It used to be pinned at `host.docker.internal:8080/alehatsman/go-quality` and
fetched from this moongit over plain http, with
`MOONCAKE_MODULE_INSECURE=host.docker.internal:8080` in `mgitci.yml` to trust
the host. That only ever worked because the image carried a warm module cache —
moongit's git endpoints require auth, and a CI container has no credentials to
answer the challenge with. Rebuilding the image emptied the cache and the gate
died at stage 1 of 11 (#192). go-quality is mirrored public on GitHub, so
pinning there removes the dependency instead of re-warming the cache.
