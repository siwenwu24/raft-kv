# Milestone 1 Report: Distributed Key-Value Store with Raft Consensus and Chaos Engineering

## Part I: Related Projects

Post 1 [here](https://piazza.com/class/mk3hftotl6e229/post/1151)

Post 2 [here](https://piazza.com/class/mk3hftotl6e229/post/1124)

Post 3 [here](https://piazza.com/class/mk3hftotl6e229/post/1119)



---

## Part II: Preliminary Results

### Local Test Setup

Run 5 instances on localhost with different ports (8081–8085) backed by Raft ports (7001–7005). Write to the leader and read from followers.


### Test Case: Write Rejection on Followers

Followers correctly reject writes and return leader information for client-side redirect:

```bash
curl -X POST http://localhost:8082/kv/test -d '{"value":"123"}'
{
    "error": "not the leader",
    "leader_addr": "127.0.0.1:7001",
    "leader_id": "node1"
}
```

### Test Case: Key Not Found

```bash
curl "http://localhost:8081/kv/nonexistent"
{
    "error": "Key not found"
}
```

### Test Case: Delete

```bash
curl -X DELETE http://localhost:8081/kv/hello
{
    "message": "Successfully deleted key - hello"
}

curl "http://localhost:8081/kv/hello"
{
    "error": "Key not found"
}
```

### Test Case: Strong Read on a Follower

Strong consistency reads are rejected on followers, enforcing linearizability:

```bash
curl "http://localhost:8082/kv/hello?level=strong"
{
    "error": "not the leader",
    "leader_addr": "127.0.0.1:7001",
    "leader_id": "node1"
}
```

### Test Case: Leader Failover

After killing node1, `node3` became the new leader and accepted writes:

```bash
curl -X POST http://localhost:8083/kv/test -d '{"value":"after-failover"}'
{
    "key": "",
    "value": "after-failover"
}
```

---

## Part III: Milestone 1 Report

### Problem, Team, and Overview of Experiments

#### Problem Statement

Modern distributed systems require strong consistency guarantees while maintaining high availability and fault tolerance. Many production systems (e.g., etcd backing Kubernetes, Consul for service discovery) rely on Raft consensus to achieve this, yet building and validating such systems remains challenging. Our project builds a distributed key-value store powered by Raft consensus and applies chaos engineering techniques to systematically verify its fault tolerance, which is an approach directly relevant to anyone operating stateful distributed infrastructure.

#### Team

| Member     |
| ---------- |
| Qian Li    |
| Zhengyi Xu |
| Wenyu Yang |
| Siwen Wu   |

In Week 1, every member independently built the entire Raft KV system end-to-end (FSM, HTTP API, Raft setup, local testing). With AI-assisted learning (Claude Code), this was feasible and gave every member a complete understanding of the full distributed systems stack.

Starting Week 2, we divide experiment and infrastructure work as follows (we haven't decided which member responsible for which part, but we will have alignment next week):

| Member | Primary Responsibility                                                          |
| ------ | ------------------------------------------------------------------------------- |
| A      | EC2 provisioning (Terraform) + structured JSON logging + data analysis (charts) |
| B      | Experiment 1: Leader Crash Recovery (chaos script + run experiment)             |
| C      | Experiment 2: Network Partition Behavior (chaos script + run experiment)        |
| D      | Experiment 3: Read Scalability + shared load generator                          |

**Dependencies:** Exp 1 and Exp 2 owners can write chaos scripts immediately. D builds the load generator, which all three experiments reuse. A's EC2 provisioning must complete before any experiment runs on AWS. After chaos scripts are written, A adds structured JSON logging into the scripts and the Go application, then builds the S3 collection and chart generation pipeline. Final report and video are shared across all members.

#### Overview of Experiments

We have designed three experiments to evaluate the system's behavior under failure:

1. **Leader Crash Recovery** — Measure election time and client-visible downtime after leader failures with varying election timeouts.
2. **Network Partition Behavior** — Validate that majority partitions continue serving, minority partitions reject writes, and no split-brain occurs.
3. **Read Scalability and Consistency Tradeoffs** — Quantify how follower reads scale horizontally and measure the latency/throughput tradeoff across strong, default, and stale consistency modes.

#### Role of AI

We used Claude Code (Anthropic's AI coding assistant) as a learning accelerator for the `hashicorp/raft` library. Since none of us had prior experience with this library, Claude Code provided step-by-step guidance on implementing the FSM interface, setting up Raft transport/storage, and understanding the subtleties of consistency levels. The actual code was written by team members. AI served as a mentor, not a co-author.

#### Observability Plan

Each node will emit structured JSON logs recording: current Raft term, node role (leader/follower/candidate), commit index, apply latency, and request throughput. Logs are collected to S3 after each experiment. A Python script processes logs and generates static analysis charts using matplotlib. We also plan to expose a `/status` endpoint on each node returning the current Raft state for real-time debugging during experiments.

### 3. Project Plan and Recent Progress

#### Timeline

| Week                  | Milestone                                                                              | Status       |
| --------------------- | -------------------------------------------------------------------------------------- | ------------ |
| Week 1 (Mar 23–29)    | Raft + KV store + HTTP API working locally, local 5-node cluster tested                | **Complete** |
| Week 2 (Mar 30–Apr 5) | AWS EC2 provisioning, deploy to AWS, chaos scripts drafted, end-to-end cluster working | Planned      |
| Week 3 (Apr 6–12)     | Run all chaos experiments, collect data to S3, begin analysis                          | Planned      |
| Week 4 (Apr 13–19)    | Generate charts, write final report, polish and present                                | Planned      |

#### Task Breakdown

**Phase 1 — Done (all members independently):**

| Task                                                      | Status             |
| --------------------------------------------------------- | ------------------ |
| FSM implementation (`Apply`, `Snapshot`, `Restore`)       | Done (all members) |
| HTTP API with GET/PUT/DELETE and 3 read consistency modes | Done (all members) |
| Raft node setup (`main.go`) with configurable timeouts    | Done (all members) |
| Local 5-node cluster testing and validation               | Done (all members) |

**Phase 2 — Week 2 (parallel work):**

| Task                                                                   | Owner | Status      |
| ---------------------------------------------------------------------- | ----- | ----------- |
| EC2 provisioning with Terraform                                        | A     | In progress |
| Exp 1 chaos script (leader kill, back-to-back kill)                    | B     | Planned     |
| Exp 2 chaos script (`iptables` partition + heal)                       | C     | Planned     |
| Load generator (configurable rate, read/write ratio, consistency mode) | D     | Planned     |

**Phase 3 — After chaos scripts done:**

| Task                                                  | Owner | Status  |
| ----------------------------------------------------- | ----- | ------- |
| Add structured JSON logging to Go app + chaos scripts | A     | Planned |
| S3 log collection pipeline                            | A     | Planned |

**Phase 4 — Week 3 (run experiments on EC2):**

| Task             | Owner | Status  |
| ---------------- | ----- | ------- |
| Run Experiment 1 | B     | Planned |
| Run Experiment 2 | C     | Planned |
| Run Experiment 3 | D     | Planned |

**Phase 5 — Week 4 (all members):**

| Task                                      | Owner       | Status  |
| ----------------------------------------- | ----------- | ------- |
| Generate charts from S3 logs (matplotlib) | A           | Planned |
| Write final report                        | All members | Planned |
| Record video                              | All members | Planned |

#### Recent Progress

In Week 1, we completed the full core implementation:

- **FSM (`fsm/fsm.go`):** Implements the `raft.FSM` interface with `Apply`, `Snapshot`, and `Restore`. Uses a mutex-protected map as the state machine. Snapshots serialize the map to JSON; restore deserializes and replaces the map atomically.
- **HTTP API (`httpd/handler.go`):** Built with the Gin framework. Supports PUT, DELETE, and GET with three read consistency levels (`?level=strong|default|stale`). Writes are leader-only; followers return a 307 redirect with leader address. Bounded-staleness reads check the gap between `LastIndex` and `AppliedIndex` against a configurable threshold.
- **Raft Setup (`cmd/kvnode/main.go`):** Configures `hashicorp/raft` with BoltDB for log/stable storage, file-based snapshots, and TCP transport. Supports environment-variable-driven configuration for node ID, bind address, data directory, election timeout, and heartbeat timeout.
- **Local Testing:** Successfully ran a 5-node cluster on localhost. Verified write replication, follower read, write rejection on followers, delete operations, strong-read enforcement, and leader failover with automatic re-election.

#### AI Cost and Benefits

- **Benefits:** Significantly accelerated the learning curve for `hashicorp/raft`. Helped identify subtle bugs (e.g., unexported methods, incorrect function signatures, missing error checks) during code review. Provided architectural guidance on consistency mode design.
- **Costs:** Required careful validation. AI suggestions occasionally needed adjustment (e.g., initially suggested silently defaulting on invalid config values, which we caught and corrected to fail-fast behavior). All code was reviewed and understood by the team before merging.

### 4. Objectives

#### Short-Term (Course Timeline)

- Deploy 5-node Raft cluster on AWS EC2 with automated provisioning
- Execute all three chaos experiments and collect quantitative data
- Produce charts and analysis demonstrating fault tolerance, consistency guarantees, and read scalability
- Deliver a complete final report with reproducible experiment methodology

#### Long-Term (Beyond Course)

- **Production hardening:** Add write forwarding (followers proxy writes to leader instead of returning redirect), request retry with backoff, and connection pooling
- **Enhanced observability:** Integrate Prometheus metrics export and Grafana dashboards for live monitoring of Raft state, commit lag, and request latency
- **Dynamic membership:** Support adding/removing nodes at runtime via Raft configuration changes, enabling rolling upgrades
- **Multi-region deployment:** Explore cross-region replication with region-aware read routing for geo-distributed workloads
- **Open-source toolkit:** Package the chaos engineering scripts as a reusable toolkit for testing any Raft-based system

### 5. Related Work

#### Course Readings

- **Raft Paper (Ongaro & Ousterhout, 2014):** Our system directly implements the Raft consensus protocol. We use `hashicorp/raft`, a production-grade Go implementation that follows the paper's design for leader election, log replication, and safety.
- **Dynamo Paper (DeCandia et al., 2007):** Dynamo uses eventual consistency with vector clocks, while our system provides stronger guarantees through Raft consensus. Our stale read mode is conceptually similar to Dynamo's eventual consistency, but our default and strong modes offer bounded and linearizable consistency.
- **CAP Theorem (Brewer, 2000):** Our three consistency modes explicitly let clients choose their position on the CAP spectrum — strong reads sacrifice availability for consistency, while stale reads sacrifice consistency for availability.


### 6. Methodology

#### System Architecture

Our system consists of 5 identical Go nodes, each running:

1. **Raft consensus layer** (`hashicorp/raft`) — handles leader election, log replication, and snapshotting
2. **FSM (Finite State Machine)** — a mutex-protected in-memory map that applies committed log entries
3. **HTTP API** (Gin framework) — client-facing REST endpoints for KV operations

```
Client Request → HTTP API → Raft Leader? → raft.Apply() → Log Replication → FSM Apply → Response
                              ↓ No
                         Return leader info (307 redirect)
```

#### Read Consistency Modes

| Mode      | Behavior                                                                                              | Tradeoff                                             |
| --------- | ----------------------------------------------------------------------------------------------------- | ---------------------------------------------------- |
| `strong`  | Must be served by the leader                                                                          | Linearizable, but all reads hit one node             |
| `default` | Served by any node within a configurable staleness threshold (`LastIndex - AppliedIndex ≤ threshold`) | Balanced — bounded staleness with horizontal scaling |
| `stale`   | Served by any node immediately                                                                        | Lowest latency, but may return stale data            |

#### Chaos Engineering Approach

We apply the principles of chaos engineering to systematically validate fault tolerance:

- **Leader kill:** SSH into the leader node and kill the process. Measure time to new election and client-visible downtime.
- **Network partition:** Use `iptables` rules to isolate node subsets. Verify majority continues, minority halts, and state converges after healing.
- **Latency injection:** Use `tc` (traffic control) to add artificial delay. Measure impact on commit latency and throughput.

#### Experiments in Detail

**Experiment 1: Leader Crash Recovery**

- 5-node cluster under 200 writes/sec
- Kill leader at t=30s
- Test with election timeouts: 150ms, 300ms, 500ms
- Also test back-to-back leader kills
- Metrics: election time, client downtime, writes lost, throughput recovery curve

**Experiment 2: Network Partition Behavior**

- Three scenarios under 200 writes/sec:
  - Minority partition (2 nodes isolated) — majority continues
  - Leader isolation — majority elects new leader
  - Symmetric split (2-2-1) — no majority, cluster halts
- After healing: correctness checker compares all 5 nodes' state
- Metrics: throughput during partition, write rejection rate, reconciliation time

**Experiment 3: Read Scalability**

- Phase A: 90% read / 10% write at 2,000 req/sec. Compare leader-only vs. follower reads with 3, 4, 5 nodes.
- Phase B: 5,000 req/sec reads with 5 nodes. Compare strong, default, stale modes.
- Metrics: throughput scaling factor, latency (p50/p95/p99), measured staleness

#### Observability

- Structured JSON logs on each node: term, role, commit index, apply latency, request count
- Logs collected to S3 post-experiment
- Python + matplotlib for static chart generation
- `/status` endpoint for real-time Raft state inspection

#### AI Usage in Methodology

Claude Code is used as a learning tool for `hashicorp/raft` API guidance, code review, experiment scripts and analysis code. All AI-assisted code is reviewed and validated before integration.

### 7. Preliminary Results

We have completed local testing of the core system. Key results:

| Test                                 | Result                                      | Validated Property                        |
| ------------------------------------ | ------------------------------------------- | ----------------------------------------- |
| Write to leader, read from followers | Passed                                      | Log replication works correctly           |
| Write rejection on followers         | Passed (returns 307 + leader info)          | Write safety — only leader accepts writes |
| Key not found                        | Passed (returns 404)                        | Correct error handling                    |
| Delete key                           | Passed                                      | DELETE operation through Raft consensus   |
| Strong read on follower              | Passed (rejected with leader info)          | Linearizable read enforcement             |
| Leader failover                      | Passed (new leader elected, accepts writes) | Automatic leader election and recovery    |

See Part II above for detailed curl output.

#### What Remains

- **Experiment 1 (Leader Crash Recovery):** Need to deploy on EC2 and run with load generator to measure election time and throughput recovery under different timeout configurations. The local failover test confirms the mechanism works, but we need quantitative timing data.
- **Experiment 2 (Network Partition):** Requires `iptables`-based partition injection on EC2. Cannot be fully tested locally. Chaos scripts are drafted.
- **Experiment 3 (Read Scalability):** Need the load generator and EC2 deployment to measure throughput scaling and latency percentiles across consistency modes.

#### Anticipated Worst-Case Workload

The pathological worst case is a **symmetric network partition (2-2-1)** combined with high write load. In this scenario, no partition has a majority, so all writes are rejected cluster-wide. If a client keeps retrying against a minority partition, it will see 100% failure rate with increasing latency as Raft election attempts time out repeatedly. This is the expected and correct behavior — Raft sacrifices availability to preserve consistency — but it represents the worst client experience.

### 8. Impact

#### Why This Matters

Distributed consensus is the backbone of modern infrastructure — Kubernetes (etcd), HashiCorp (Consul/Nomad), and CockroachDB all rely on Raft. Understanding how these systems behave under failure is critical for anyone building or operating cloud-native applications. Our project provides:

1. **A clear, minimal Raft KV implementation** that others can study to understand consensus internals without the complexity of production systems like etcd.
2. **Reproducible chaos experiments** with quantitative results showing exactly how Raft handles leader crashes, network partitions, and degraded networks.
3. **Empirical data on the consistency-throughput tradeoff** — a question every distributed system designer faces.

#### Community Involvement

We welcome classmates to test our system once deployed on EC2. The HTTP API is simple (just curl), and we can provide a public endpoint for read experiments. If other teams are building distributed systems (e.g., the projects we referenced on Piazza), comparing fault tolerance results across different consensus approaches would be valuable.

---

## Part IV: Video Script

https://drive.google.com/drive/folders/1NqGJZICWFdXzglFuW-2AjwlP-o-JgTAN


