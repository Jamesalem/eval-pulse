---
name: "Bug Report"
about: "Report a reproducible defect, rate-limiting issue, or worker failure in EvalPulse"
title: "[BUG]: "
labels: ["bug", "triage"]
assignees: ""
---

### 1. Description & Context
Provide a clear and concise description of what the bug is.

### 2. Affected Component
- [ ] Go API Gateway (`cmd/api`)
- [ ] Go Worker Pool (`cmd/worker`)
- [ ] Redis Stream & Consumer Group (`internal/queue`)
- [ ] Evaluation Heuristics / Scoring (`internal/eval`)
- [ ] Provider Adapters (Gemini / Claude / OpenAI / Ollama)
- [ ] React UI Dashboard (`web/`)
- [ ] Docker Compose Orchestration

### 3. Steps to Reproduce
1. Run command '...'
2. Submit eval payload with '...'
3. Inspect Redis Stream or UI at '...'
4. See error

### 4. Expected vs Actual Behavior
- **Expected:** Worker acknowledges Redis stream message and computes score.
- **Actual:** Worker drops message or encounters unhandled panic / rate limit.

### 5. Environment & Logs
- **OS:** Windows / Linux / macOS
- **Go Version:** `go version`
- **Docker / Redis Version:**
- **Worker / API Log Output:**
```text
(Paste relevant logs here)
```
