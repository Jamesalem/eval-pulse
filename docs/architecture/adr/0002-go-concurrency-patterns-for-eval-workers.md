# ADR 0002: Go Concurrency Patterns for Distributed Evaluation Workers

| Metadata | Information |
| :--- | :--- |
| **Status** | Accepted |
| **Date** | 2026-09-22 |
| **Deciders** | Principal Systems Architect, Lead Backend Engineer |
| **Technical Context** | Go Worker Pool Concurrency & Rate Throttling |

---

## Context and Problem Statement

Evaluation workers in EvalPulse must execute concurrent outbound network calls to external frontier LLMs (Gemini, Claude, OpenAI) and local instances (Ollama). LLM API endpoints enforce strict Rate Limits (Requests Per Minute - RPM, and Tokens Per Minute - TPM).

If a worker spawns unbounded goroutines for every incoming benchmark task, the following failure modes occur:
1. **Upstream 429 Throttling:** Massive burst traffic triggers rate-limit cascades and dropped evaluation suites.
2. **Goroutine Leaks & Memory Inflation:** Orphaned network calls fail to release socket descriptors and heap buffers, violating our < 150MB RSS memory ceiling.
3. **Imperfect Teardown:** Terminating a worker node mid-run causes half-evaluated suites and missing acknowledgments.

We evaluated three concurrency architectures in Go: **Bounded Semaphore Worker Pool**, **Unbounded Goroutines with Mutexes**, and the **Actor Model / Asynq**.

---

## Decision Drivers

- **Rate-Limit Respect & Backpressure:** Strict bounding of simultaneous outbound requests.
- **Resource Predictability:** Bounded memory overhead and zero goroutine leaks.
- **Graceful Teardown:** Complete in-flight evaluations upon `SIGTERM` within a 30-second drain window.
- **Minimal Dependencies:** Leverage standard Go concurrency primitives (`sync.WaitGroup`, channels, `context`).

---

## Considered Options

### Option 1: Bounded Worker Pool with Buffered Semaphore Channels (Chosen)
A worker pool initialized with a buffered channel acting as a counting semaphore:
```go
type WorkerPool struct {
    sem chan struct{}
    wg  sync.WaitGroup
    ctx context.Context
}
```
- **Pros:**
  - Strict ceiling on concurrent goroutines (e.g., `cap(sem) = 50`).
  - Natural backpressure: workers block on Redis stream dequeue when semaphore is saturated.
  - Zero external third-party dependencies (100% standard Go runtime).
  - Trivial context cancellation propagation (`ctx.Done()`).
- **Cons:**
  - Dynamic scaling requires atomic resizing or channel replacement.

### Option 2: Unbounded Goroutines with Rate-Limit Sleeping
Spawning `go worker(task)` for every message and relying on sleep timers.
- **Pros:**
  - Simple initial implementation.
- **Cons:**
  - High risk of memory spikes under burst loads (> 10,000 tasks).
  - No guaranteed bounds on socket descriptors.
  - Fragile graceful shutdown semantics.

### Option 3: Distributed Actor Framework (e.g., ProtoActor or Asynq)
Adopting an actor model or heavy distributed task library.
- **Pros:**
  - Built-in mailbox and supervision trees.
- **Cons:**
  - Significant architectural overhead and steep learning curve for contributors.
  - Redundant with Redis Streams consumer groups already handling message distribution.

---

## Decision Outcome

**Chosen Option: Option 1 (Bounded Worker Pool with Buffered Semaphore Channels)**

By using a bounded counting semaphore paired with `sync.WaitGroup` and Go's `context.Context`, EvalPulse achieves deterministic concurrency control, clean backpressure propagation to the Redis Stream consumer group, and predictable memory utilization (< 150 MB RSS).

---

## Implementation Rules & Safeguards

1. **Context Propagation:** Every outbound HTTP request must inherit the worker context with an explicit timeout (`context.WithTimeout(ctx, 30*time.Second)`).
2. **Body Closure:** Every HTTP response body must be closed immediately via `defer resp.Body.Close()` to prevent TCP socket exhaustion.
3. **Graceful Draining:** Upon receiving `SIGINT` or `SIGTERM`, the main routine stops Redis polling, cancels the root context, and waits on `wg.Wait()` before exiting with code 0.
