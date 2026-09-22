# ADR 0001: Redis Streams vs. Apache Kafka vs. RabbitMQ for Distributed Evaluation Orchestration

| Metadata | Information |
| :--- | :--- |
| **Status** | Accepted |
| **Date** | 2026-09-22 |
| **Deciders** | Principal Systems Architect, Lead Backend Engineer, Lead TPM |
| **Technical Context** | Distributed Job Orchestration & Asynchronous State Machine |

---

## Context and Problem Statement

EvalPulse coordinates distributed, asynchronous benchmark evaluations across frontier and local LLMs. The platform requires a messaging backbone capable of:
1. Handling bursty, high-throughput job submissions from CI/CD pipelines and the React UI.
2. Providing distributed consumer groups with at-least-once delivery guarantees, explicit message acknowledgment (`ACK`), and dead-letter queueing.
3. Enabling low-latency task dequeue (< 5ms) without operational overhead, heavy JVM dependencies, or complex multi-node cluster maintenance on developer workstations.

We evaluated three architectural candidates: **Redis Streams**, **Apache Kafka**, and **RabbitMQ**.

---

## Decision Drivers

- **Developer Onboarding & Operational Simplicity:** Must be trivial to spin up locally via Docker Compose (< 50MB RAM footprint).
- **Sub-Millisecond Dequeue Latency:** Low latency for time-sensitive interactive evaluations.
- **Consumer Group Support:** Native consumer groups with pending entries list (PEL) and automated message claiming (`XAUTOCLAIM`).
- **Data Persistence & Unified Store:** Dual-use capability as both a streaming queue and a high-performance in-memory metrics cache.

---

## Considered Options

### Option 1: Redis Streams (Chosen)
Redis Streams (introduced in Redis 5.0) provides an append-only log data structure with consumer groups, persistent offset tracking, and atomic acknowledgment primitives (`XACK`, `XPENDING`, `XAUTOCLAIM`).

- **Pros:**
  - Lightweight single binary with ultra-low memory footprint (< 30 MB base).
  - Sub-millisecond read/write latencies.
  - Native consumer groups with automatic worker failover (`XAUTOCLAIM`).
  - Co-locates evaluation job streams and metric caching in the same operational container.
  - Trivial local and CI/CD integration.
- **Cons:**
  - In-memory constraint requires stream trimming (`MAXLEN ~ 100000`) for high-volume retention.

### Option 2: Apache Kafka
Enterprise distributed event streaming platform based on partitioned commit logs.

- **Pros:**
  - Infinite disk-backed event retention and replayability.
  - Unmatched multi-datacenter horizontal scale.
- **Cons:**
  - Heavy operational complexity (ZooKeeper or KRaft metadata management).
  - High baseline memory overhead (> 1.5 GB RAM), making local single-laptop evaluation burdensome.
  - Higher consumer poll latency relative to in-memory Redis.

### Option 3: RabbitMQ (AMQP)
Traditional message broker with flexible routing exchanges and AMQP protocol support.

- **Pros:**
  - Mature routing topologies and dead-letter exchanges.
- **Cons:**
  - Message replay is difficult; messages are destroyed once acknowledged.
  - Requires Erlang runtime and separate storage layer for metrics caching.

---

## Decision Outcome

**Chosen Option: Option 1 (Redis Streams)**

Redis Streams provides the ideal balance between distributed reliability, high performance, and operational simplicity for EvalPulse. By coupling Redis Streams for queueing with Redis Hashes/Sorted Sets for real-time metric storage, EvalPulse reduces its external infrastructure dependency to a single, high-performance container.

---

## Consequences

- **Positive:**
  - Rapid local setup: developer onboarding takes less than 60 seconds with `docker compose up`.
  - Zero JVM or Erlang overhead; streamlined CI/CD pipeline in GitHub Actions.
  - Consumer group semantics ensure zero job loss even if worker nodes restart mid-benchmark.
- **Negative / Trade-offs:**
  - Memory capacity must be monitored; streams are capped using `MAXLEN ~ 50000` to prevent unbounded memory growth. Long-term metric archives are persisted to SQLite/PostgreSQL.
