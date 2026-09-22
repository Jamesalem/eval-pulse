# Contributing to EvalPulse

Thank you for your interest in contributing to **EvalPulse**! We welcome contributions from the community to improve distributed LLM evaluation, add new provider connectors, and optimize regression detection heuristics.

---

## 1. Development Principles & Code Standards

- **Go Standards:**
  - Code must adhere to standard Go conventions (`gofmt`, `go vet`).
  - Strict bounded concurrency: No unbound goroutine spawning. Outbound HTTP requests must propagate contexts with timeouts (`context.WithTimeout`).
  - Zero memory leaks: Worker resident memory must remain `< 150 MB RSS`.
- **Frontend Standards:**
  - Modern React 18+ functional components with Tailwind CSS.
  - No bloated state management libraries; state is coordinated through lightweight reactive hooks.
- **Commit Conventions:**
  - Follow Conventional Commits:
    - `feat(eval): add semantic similarity metric`
    - `fix(queue): handle consumer group BUSYGROUP error`
    - `docs(sdd): update sequence diagram`
    - `perf(worker): optimize channel semaphore backpressure`

---

## 2. Local Setup & Testing

### Prerequisites
- Docker & Docker Compose (or Go 1.22+ and Node.js 20+)
- Redis 7.2+ (optional for local standalone mode)

### Running Tests
```bash
# Run all Go unit tests with race detection
make test

# Build and verify React dashboard
cd web
npm install
npm run build
```

---

## 3. Pull Request Checklist

Before submitting a Pull Request, ensure:
1. [ ] Code compiles cleanly with zero warnings or linter errors.
2. [ ] Unit test coverage across modified packages is $\ge 80\%$.
3. [ ] All API changes are reflected in `docs/api/openapi.yaml`.
4. [ ] Architectural changes include an updated ADR in `docs/architecture/adr/`.
