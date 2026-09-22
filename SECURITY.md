# Security Policy: EvalPulse

## 1. Zero-Exfiltration Security Philosophy

EvalPulse is architected from the ground up for zero-trust data privacy and zero API key exfiltration:
- **Client-Side Credential Retention:** API keys and provider access tokens (Gemini, Claude, OpenAI, OpenRouter) inputted in the UI are retained exclusively within browser local storage and ephemeral container memory during active benchmark runs.
- **Zero Cloud Intermediaries:** Outbound HTTP requests to model providers are made directly from the user's workstation or private container runtime. No evaluation payloads or credentials pass through any external hosted EvalPulse infrastructure.
- **Zero Plaintext Logging:** Sensitive headers (`x-api-key`, `Authorization: Bearer ...`) are redacted before job events are pushed to Redis Streams or log sinks.

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
