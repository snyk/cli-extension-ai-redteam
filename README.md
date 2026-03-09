# Snyk AI Red Team CLI Extension

## Overview

Implements running red teaming scans against LLM-based applications via `snyk redteam`. This CLI, written in Go, deprecates the Python CLI.

---

## Quick setup

### Prerequisites

- [Snyk CLI](https://docs.snyk.io/snyk-cli/install-the-snyk-cli) and running `snyk auth` once
- For **pre-prod** only: [Teleport (tsh)](https://goteleport.com/docs/accessing-clusters/tsh/) and access to the pre-prod cluster

### Target app (both scenarios)

Use the [ai-red-teaming-benchmark](https://github.com/snyk/ai-red-teaming-benchmark) as the scan target. Run it locally:

```bash
git clone https://github.com/snyk/ai-red-teaming-benchmark
cd ai-red-teaming-benchmark
# Set ANTHROPIC_API_KEY if required by the scenario
make run
```

The benchmark typically serves on **[http://localhost:8000](http://localhost:8000)** with scenario paths like `/scenarios/chatbot_gpt_4o` or `/scenarios/chatbot_claude_sonnet_4_5`. The API expects a JSON body with `message` and `session_id`.

---

### Scenario 1: Minired backend running locally (Tilt)

1. In the **minired** repo, start the backend with Tilt:
  ```bash
   tilt up
  ```
   Minired will be available at **[http://localhost:8085](http://localhost:8085)**.
2. In the **cli-extension-ai-redteam** directory, create or use a `redteam.yaml` (see sample below) with `control_server_url: "http://localhost:8085"`.
3. Run a scan:
  ```bash
   make redteam -- --experimental
  ```

---

### Scenario 2: Minired backend in pre-prod (Teleport port-forward)

1. Log in to Teleport (if not already):
  ```bash
   tsh login --proxy=snyk.teleport.sh:443
  ```
2. From the **minired** repo, start the port-forward to pre-prod (keep this running):
  ```bash
   cd minired-sdk && ./port-forward.sh 8085
  ```
   This forwards **localhost:8085** to the minired deployment in the pre-prod cluster.
3. In **cli-extension-ai-redteam**, use a `redteam.yaml` with `control_server_url: "http://localhost:8085"` (same as local Tilt).
4. In another terminal, run the scan:
  ```bash
   cd cli-extension-ai-redteam
   make redteam -- --experimental
  ```

---

### Sample `redteam.yaml` (both scenarios)

The same config works for both scenarios when the control server is reached at **[http://localhost:8085](http://localhost:8085)** (local Tilt or pre-prod port-forward). Only the target URL/path may change depending on which benchmark scenario you use.

```yaml
target:
  name: benchmark-local
  type: api
  context:
    purpose: red team scan
  settings:
    url: "http://localhost:8000/scenarios/chatbot_claude_sonnet_4_5"
    response_selector: "response"
    request_body_template: '{"message": "{{prompt}}", "session_id": "redteam-session"}'

control_server_url: "http://localhost:8085"
goal: system_prompt_extraction
strategies:
  - directly_asking
```

**Scenario 1 (local Tilt):** use as-is; minired is already on port 8085.

**Scenario 2 (pre-prod):** use as-is after starting `./port-forward.sh 8085` in [minired-sdk](https://github.com/snyk/minired/tree/main/minired-sdk); the forward exposes pre-prod minired on localhost:8085.