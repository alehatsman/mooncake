# mooncake CI image — the toolchain mooncake's own mgitci.yml runs on.
#
# moongit runs each CI job in a throwaway container and execs `mooncake step`
# inside it (see moongit's cmd/moongitd/ci_runner.go), so the image must carry
# `mooncake` + git on PATH plus whatever the steps shell out to.
#
# `mooncake task ci` is an 11-step gate. Its Go tooling
# (golangci-lint/govulncheck/gocyclo/dupl), go + python3 + a static `mooncake`
# all come from moongit-ci-dev:latest, which this derives from. The one thing
# that base lacks is `gomarkdoc`, which the docs-regen step (6/11) shells out to
# — so this image adds it. (The mkdocs step (7/11) self-skips when pipenv/mkdocs
# are absent, so they are deliberately NOT installed here; add them in a derived
# image if you want that step to actually build the site.)
#
# gomarkdoc is pinned to match tasks.yml's docs-tools-install (v0.4.1) so CI and
# the local docs pipeline render byte-identical output.
#
# Build (requires moongit-ci-dev:latest to exist; see moongit/ci/README.md):
#   docker build -t mooncake-ci:latest ci/
#
# Go toolchain: the base (moongit-ci-dev:latest) ships go1.26.3 with
# GOTOOLCHAIN=local, which hard-errors against go.mod's `go 1.26.4` directive
# (bumped for the GO-2026-5037/5039 stdlib fixes that govulncheck, step 5/11,
# enforces). The base has no curl/wget to fetch a tarball, so we overlay GOROOT
# from the official golang:1.26.4 image. When the moongit-ci-dev base itself
# moves to >=1.26.4, this overlay becomes a redundant no-op and can be dropped.
FROM golang:1.26.4 AS gotoolchain

FROM moongit-ci-dev:latest

# Overlay go1.26.4 over the base's go1.26.3 GOROOT (see header). go-quality
# binaries (golangci-lint/govulncheck/…) live outside GOROOT and are untouched;
# govulncheck reads this GOROOT's stdlib version, so the fixes are picked up.
COPY --from=gotoolchain /usr/local/go /usr/local/go

ARG GOMARKDOC_VERSION=v0.4.1

# Install gomarkdoc onto PATH, then wipe the build-time caches so runtime jobs
# (which run as the moongitd uid) own the cache dirs they recreate — root-owned
# dirs would be unwritable. Mirrors moongit-ci-dev:latest's cache hygiene.
RUN GOBIN=/usr/local/bin go install \
      "github.com/princjef/gomarkdoc/cmd/gomarkdoc@${GOMARKDOC_VERSION}" \
 && go clean -cache -modcache \
 && rm -rf /tmp/go /tmp/.cache

RUN mooncake --version && go version && gomarkdoc --version
