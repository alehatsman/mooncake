# Mooncake Presets Index

**Total: 384 Production-Ready Presets**

All presets include:
- ✅ preset.yml (parameter definitions)
- ✅ tasks/install.yml (cross-platform installation)
- ✅ tasks/uninstall.yml (clean removal)
- ✅ README.md (usage documentation)
- ✅ Agent-friendly (CLI-based, scriptable)

## Quick Reference

### Data Processing (30+)
- **JSON**: jq, yq, jless, gron, fx, jd
- **CSV**: miller, xsv, csvkit, csvtool
- **YAML**: yq, yaml-cli
- **XML**: xmllint, xmlstarlet

### Containers & Kubernetes (40+)
- **Container Tools**: docker, docker-compose, podman, nerdctl
- **K8s Tools**: kubectl, helm, helmfile, k9s, kubectx, kubens
- **Image Tools**: dive, skopeo, crane, buildah
- **Security**: trivy, syft, grype, cosign, aqua, snyk

### Cloud Providers (25+)
- **Major**: awscli, gcloud, azure-cli
- **Regional**: doctl, linode-cli, vultr-cli, hcloud
- **Platform**: fly, render, railway, vercel
- **Multi**: oci-cli, ibmcloud-cli, scaleway-cli

### Databases (40+)
- **SQL**: mycli, pgcli, litecli, usql
- **NoSQL**: mongosh, redis-cli, cassandra
- **Time-Series**: influx-cli, questdb, timescaledb
- **Graph**: neo4j, arangodb, dgraph
- **Vector**: weaviate, qdrant, pinecone, milvus

### Languages & Runtimes (50+)
- **JavaScript**: nvm, volta, fnm, pnpm, yarn, bun, deno
- **Python**: poetry, pipenv, pdm, hatch, rye, uv
- **Rust**: rustup, cargo-edit, cargo-watch
- **Go**: golangci-lint, goreleaser, air
- **Java**: sdkman, maven, gradle, ant
- **Ruby**: rbenv, rvm, chruby

### Monitoring & Observability (30+)
- **Metrics**: prometheus, victoria-metrics, mimir
- **Tracing**: tempo, jaeger, zipkin
- **Logging**: loki, fluentd, vector
- **APM**: elastic-apm, datadog-agent, newrelic
- **Profiling**: pyroscope, parca

### Message Queues (20+)
- **Streaming**: kafka, pulsar, redpanda
- **Queue**: rabbitmq, activemq, nats
- **Pub/Sub**: mqtt, mosquitto

### CI/CD & GitOps (30+)
- **CI**: act, gitlab-runner, circleci-cli, buildkite
- **CD**: argocd, flux, spinnaker, harness
- **Workflows**: argo-workflows, tekton, drone

### Infrastructure as Code (25+)
- **Core**: terraform, pulumi, crossplane
- **Tools**: terragrunt, tflint, tfsec, infracost
- **CDK**: cdk, cdktf

### Security & Secrets (30+)
- **Scanning**: trivy, grype, snyk, aqua
- **Secrets**: vault, sops, age, doppler
- **Password**: 1password-cli, bitwarden-cli, pass
- **Compliance**: checkov, polaris, kube-bench

### Networking (20+)
- **Diagnostic**: mtr, iperf3, gping, dog
- **HTTP**: httpie, xh, curlie, httpstat
- **Load**: vegeta, wrk, hey, ab
- **Scan**: nmap, masscan, rustscan

### Shell & Terminal (15+)
- **Shells**: fish, nushell, elvish, powershell
- **Terminals**: alacritty, kitty, wezterm
- **Prompts**: starship, oh-my-posh, p10k
- **Multiplexer**: tmux, screen, byobu, zellij

### Editors & IDEs (15+)
- **CLI**: neovim, helix, kakoune, micro
- **GUI**: vscode, vscodium, sublime
- **Heavy**: intellij, pycharm, goland

### Static Sites (15+)
- **Generators**: hugo, jekyll, gatsby, next
- **Docs**: mkdocs, docusaurus, vuepress
- **Minimal**: eleventy, astro, pelican

### File Transfer & Backup (15+)
- **Cloud Sync**: rclone, s3cmd, gsutil
- **Backup**: restic, borg, duplicity
- **Transfer**: croc, magic-wormhole, transfer-sh

### System Utilities (30+)
- **Monitoring**: htop, btop, bottom, glances
- **Process**: procs, pgrep, killall
- **Disk**: ncdu, duf, dust, df
- **Network**: netstat, ss, lsof

### Development Tools (40+)
- **Build**: make, cmake, ninja, bazel, meson
- **Task**: just, task, watchexec, entr
- **Version**: asdf, rtx, mise
- **Lint**: shellcheck, hadolint, yamllint

## Usage Patterns

### Agent Workflows

```yaml
# Data pipeline
- preset: jq          # Parse JSON
- preset: yq          # Convert YAML
- preset: csvkit      # Transform CSV

# Deploy application  
- preset: docker      # Build image
- preset: trivy       # Scan security
- preset: cosign      # Sign image
- preset: kubectl     # Deploy
- preset: helm        # Install chart

# Infrastructure
- preset: terraform   # Provision
- preset: ansible     # Configure
- preset: prometheus  # Monitor

# Development
- preset: gh          # GitHub ops
- preset: act         # Test actions
- preset: argocd      # Deploy
```

### Platform Setup

```yaml
# Cloud-native stack
- preset: kubectl
- preset: helm  
- preset: k9s
- preset: argocd
- preset: flux
- preset: prometheus
- preset: grafana
- preset: loki

# Developer workstation
- preset: git
- preset: gh
- preset: docker
- preset: kubectl
- preset: terraform
- preset: ansible
- preset: awscli
- preset: gcloud
```

## Directory Structure

```
presets/
├── jq/
│   ├── preset.yml
│   ├── README.md
│   └── tasks/
│       ├── install.yml
│       └── uninstall.yml
├── [383 more presets...]
└── PRESETS_INDEX.md (this file)
```

## Contributing

When adding new presets:
1. Follow existing structure (preset.yml + tasks/ + README.md)
2. Keep READMEs compact (<100 lines)
3. Include usage examples
4. Make it agent-friendly (CLI, exit codes, JSON output)
5. Test on macOS and Linux
6. Document parameters

## Quality Standards

Each preset must:
- ✅ Install cleanly on macOS (Homebrew)
- ✅ Install cleanly on Linux (apt/dnf/manual)
- ✅ Uninstall without leaving artifacts
- ✅ Be idempotent (safe to run multiple times)
- ✅ Have clear error messages
- ✅ Support automation (no interactive prompts by default)
- ✅ Include version verification
- ✅ Document common use cases

## Testing

```bash
# Test a preset
./out/mooncake run -c test.yml

# Where test.yml:
# - preset: tool-name
#   register: result
# - assert:
#     command:
#       cmd: "tool-name --version"
#       exit_code: 0
```

---

**Last Updated**: 2026-02-06  
**Total Presets**: 384  
**Status**: Production Ready ✅
