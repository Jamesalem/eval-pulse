# Security Policy: EvalPulse

## 1. Zero-Exfiltration Security Philosophy

EvalPulse is architected from the ground up for zero-trust data privacy and zero API key exfiltration:
- **Credential Lifecycle:** API keys (Gemini, Claude, OpenAI) entered in the UI are stored in the browser's local storage and sent only to your own EvalPulse API, and only the keys needed by the models selected for a run. In distributed mode, keys travel inside the job message on the Redis stream; the worker deletes the message (`XDEL`) as soon as the job is acknowledged. Keys are never written to job records, results, or the metrics store.
- **Dead-Letter Redaction:** Messages moved to the dead-letter stream have credentials stripped; unparseable payloads are dropped rather than copied.
- **No Keys in URLs or Errors:** Gemini keys are sent in the `x-goog-api-key` header rather than the query string, and transport errors are stripped of request URLs, so keys cannot surface in logs or stored error messages.
- **Zero Cloud Intermediaries:** Outbound HTTP requests to model providers are made directly from the user's workstation or private container runtime. No evaluation payloads or credentials pass through any external hosted EvalPulse infrastructure.
- **No Request-Body Logging:** The API logs method, path, status and duration only. Request bodies, which carry credentials, are never logged.
- **Input Limits:** Request bodies are capped at 1 MiB; prompt size, model count, `max_tokens`, and per-job budget are validated server-side, and user-supplied endpoints must be absolute `http(s)` URLs.

---

## 2. Supported Versions

Security updates are actively applied to the following release tracks:

| Version | Supported | Notes |
| :--- | :---: | :--- |
| **1.0.x (GA)** | ✅ | Current General Availability Release |
| < 1.0.0 | ❌ | Development Pre-releases |

---

## 3. Reporting a Vulnerability

We take the security of distributed AI evaluation infrastructure extremely seriously. If you discover a vulnerability or security risk, please follow our coordinated disclosure process:

1. **Do NOT open a public GitHub issue.** Public disclosure exposes production pipelines before a patch is available.
2. **Email Disclosure:** Send details to `security@evalpulse.dev` (or open a Private Security Advisory via GitHub's Security tab).
3. **Information to Include:**
   - Detailed description of the vulnerability.
   - Steps to reproduce or proof-of-concept payload.
   - Affected components (`cmd/api`, `cmd/worker`, `internal/queue`, `web/`).
   - Potential impact on Redis stream state or credential handling.

---

## 4. Response Timeframes & SLA

- **Initial Response:** Within 24 hours of report receipt.
- **Triage & Reproduction:** Within 48 hours.
- **Patch Deployment:** Critical severity vulnerabilities are patched and released within 7 calendar days under a tracked CVE.
