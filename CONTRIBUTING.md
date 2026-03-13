# Contributing

This repo is intended for internal (Snyk) contributions only at this time.

## Getting Started

### Prerequisites

- Go 1.22+
- Python 3.x (for pre-commit hooks)

### Install tools

```bash
make install-tools
```

This installs golangci-lint, pre-commit hooks, and Go tool dependencies.

### Authenticate

The CLI requires Snyk authentication. Run this once before your first scan:

```bash
go run cmd/develop/main.go auth
```

When working against pre-prod, export the API URL first:

```bash
export SNYK_API=https://api.dev.snyk.io
go run cmd/develop/main.go auth
```

### Run a scan

```bash
go run cmd/develop/main.go redteam --experimental --config=targets/minimal.yaml \
  --target-url=https://your-app.example.com/api/chat \
  --headers 'Authorization: Bearer <token>'
```

### Target config

Create a YAML file in `targets/` (see `targets/minimal.yaml` for reference):

```yaml
target:
  name: my-app
  type: api
  settings:
    url: "https://your-app.example.com/api/chat"
    response_selector: "response"
    request_body_template: '{"message": "{{prompt}}"}'

goal: "system_prompt_extraction"
strategies:
  - "directly_asking"
```

Secrets (auth headers, tokens) should be passed via `--headers` on the command line, not in config files.

### Common tasks

| Task | Command |
|------|---------|
| Run tests | `make test` |
| Lint | `make lint` |
| Auto-fix lint | `make format` |
| Generate mocks | `make generate` |
