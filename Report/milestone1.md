Milestone 1 Report: Distributed Key-Value Store with Raft Consensus and Chaos Engineering

Part I: Related Projects

Sharded Distributed KV Store: https://piazza.com/class/mk3hftotl6e229/post/1124
Fault-Tolerant Distributed Rate Limiter: https://piazza.com/class/mk3hftotl6e229/post/1119
High-Availability L7 Load Balancer: https://piazza.com/class/mk3hftotl6e229/post/957


Part II: Preliminary Results
Local Test Setup
Run 3 instances on localhost with HTTP ports 8000–8002 backed by Raft ports 7000–7002. Write to the leader and read from followers.
Test Case: Write to Leader
curl -X PUT http://127.0.0.1:8000/kv/hello -d '{"value":"world"}'
{"key":"hello","value":"world"}
Test Case: Strong Read (Leader Only)
curl "http://127.0.0.1:8000/kv/hello?level=strong"
{"consistency":"strong","key":"hello","last_applied_at":"2026-03-29T23:45:42","value":"world"}
Test Case: Bounded Staleness Read Rejected on Stale Follower
curl "http://127.0.0.1:8001/kv/hello?level=default"
{"error":"node is too stale for default-consistency read"}
Node2 detected its LastAppliedAt exceeded the 500ms threshold and correctly refused to serve.
Test Case: Stale Read on Follower
curl "http://127.0.0.1:8002/kv/hello?level=stale"
{"consistency":"stale","key":"hello","last_applied_at":"2026-03-29T23:45:42","value":"world"}
Test Case: Delete
curl -X DELETE http://127.0.0.1:8000/kv/hello
{"deleted":true,"key":"hello"}

curl "http://127.0.0.1:8000/kv/hello?level=stale"
{"error":"not found","key":"hello"}
Test Case: Transparent Write Forwarding from Follower
curl -X PUT http://127.0.0.1:8001/kv/test -d '{"value":"from-follower"}'
{"key":"test","value":"from-follower"}

curl "http://127.0.0.1:8000/kv/test?level=strong"
{"consistency":"strong","key":"test","value":"from-follower"}
Node2 (follower) transparently forwarded the write to the leader. Strong read on the leader confirmed the data was committed.
Test Case: Leader Failover
# Before: node1 = Leader, term=2
curl http://127.0.0.1:8000/health
{"leader_id":"node1","node_state":"Leader","term":"2"}

# After killing node1:
curl http://127.0.0.1:8001/health
{"leader_id":"node2","node_state":"Leader","term":"4"}

# Pre-failover data survived:
curl "http://127.0.0.1:8001/kv/test?level=strong"
{"consistency":"strong","key":"test","value":"from-follower"}

# New leader accepts writes:
curl -X PUT http://127.0.0.1:8001/kv/afterfailover -d '{"value":"still-works"}'
{"key":"afterfailover","value":"still-works"}
After killing node1, node2 was elected as the new leader (term advanced from 2 to 4). Pre-failover data survived and new writes were accepted immediately.

Part III: Milestone 1 Report
1. Repository
Code: https://github.com/siwenwu24/raft-kv

2. Problem, Team, and Overview of Experiments
Problem Statement
Modern distributed systems require strong consistency guarantees while maintaining high availability and fault tolerance. Many production systems (e.g., etcd backing Kubernetes, Consul for service discovery) rely on Raft consensus to achieve this, yet building and validating such systems remains challenging. Our project builds a distributed key-value store powered by Raft consensus and applies chaos engineering techniques to systematically verify its fault tolerance — an approach directly relevant to anyone operating stateful distributed infrastructure.
Team
In Week 1, every member independently built the entire Raft KV system end-to-end (FSM, HTTP API, Raft setup, local testing). With AI-assisted learning (Claude Code), this was feasible and gave every member a complete understanding of the full distributed systems stack.
Starting Week 2, we divide experiment and infrastructure work as follows:
MemberPrimary ResponsibilitySiwen WuEC2 provisioning (Terraform) + structured JSON logging + data analysis (charts)Qian LiExperiment 1: Leader Crash Recovery (chaos script + run experiment)Zhengyi XuExperiment 2: Network Partition Behavior (chaos script + run experiment)Wenyu YangExperiment 3: Read Scalability + shared load generator
Dependencies: Exp 1 and Exp 2 owners can write chaos scripts immediately. Wenyu builds the load generator, which all three experiments reuse. Siwen's EC2 provisioning must complete before any experiment runs on AWS. After chaos scripts are written, Siwen adds structured JSON logging into the scripts and the Go application, then builds the S3 collection and chart generation pipeline. Final report and video are shared across all members.
Overview of Experiments

Leader Crash Recovery — Measure election time and client-visible downtime after leader failures with varying election timeouts.
Network Partition Behavior — Validate that majority partitions continue serving, minority partitions reject writes, and no split-brain occurs.
Read Scalability and Consistency Tradeoffs — Quantify how follower reads scale horizontally and measure the latency/throughput tradeoff across strong, default, and stale consistency modes.

Role of AI
We used Claude Code (Anthropic's AI coding assistant) as a learning accelerator for the hashicorp/raft library. Since none of us had prior experience with this library, Claude Code provided step-by-step guidance on implementing the FSM interface, setting up Raft transport/storage, and understanding the subtleties of consistency levels. The actual code was written by team members. AI served as a mentor, not a co-author.
Observability Plan
Each node will emit structured JSON logs recording: current Raft term, node role (leader/follower/candidate), commit index, apply latency, and request throughput. Logs are collected to S3 after each experiment. A Python script processes logs and generates static analysis charts using matplotlib. A /health endpoint on each node returns the current Raft state for real-time debugging during experiments.

3. Project Plan and Recent Progress
Timeline
WeekMilestoneStatusWeek 1 (Mar 24–30)Raft + KV store + HTTP API working locally, local cluster tested✅ CompleteWeek 2 (Mar 31–Apr 6)AWS EC2 provisioning, deploy to AWS, chaos scripts drafted🔄 In progressWeek 3 (Apr 7–13)Run all chaos experiments, collect data to S3, begin analysis⬜ PlannedWeek 4 (Apr 14–20)Generate charts, write final report, polish and present⬜ Planned
Task Breakdown
Phase 1 — Done (all members independently):
TaskStatusFSM implementation (Apply, Snapshot, Restore)Done (all members)HTTP API with GET/PUT/DELETE and 3 read consistency modesDone (all members)Raft node setup (main.go) with configurable timeoutsDone (all members)Local cluster testing and validationDone (all members)
Phase 2 — Week 2 (parallel work):
TaskOwnerStatusEC2 provisioning with TerraformSiwenIn progressExp 1 chaos script (leader kill, back-to-back kill)QianPlannedExp 2 chaos script (iptables partition + heal)ZhengyiPlannedLoad generator (configurable rate, read/write ratio, consistency mode)WenyuPlanned
Phase 3 — After chaos scripts done:
TaskOwnerStatusAdd structured JSON logging to Go app + chaos scriptsSiwenPlannedS3 log collection pipelineSiwenPlanned
Phase 4 — Week 3:
TaskOwnerStatusRun Experiment 1QianPlannedRun Experiment 2ZhengyiPlannedRun Experiment 3WenyuPlanned
Phase 5 — Week 4:
TaskOwnerStatusGenerate charts from S3 logs (matplotlib)SiwenPlannedWrite final reportAll membersPlannedRecord videoAll membersPlanned
Recent Progress
In Week 1, we completed the full core implementation:

FSM (fsm/fsm.go): Implements the raft.FSM interface with Apply, Snapshot, and Restore. Uses a mutex-protected map as the state machine. Snapshots serialize the map to JSON; Restore deserializes and replaces the map atomically. Tracks LastAppliedAt timestamp on every commit.
HTTP API (httpd/handler.go): Built with the Gin framework. Supports PUT, DELETE, and GET with three read consistency levels (?level=strong|default|stale). Writes are transparently forwarded to the leader. Bounded-staleness reads check LastAppliedAt against a configurable threshold.
Raft Setup (cmd/kvnode/main.go): Configures hashicorp/raft with BoltDB for log/stable storage, file-based snapshots, and TCP transport. Supports environment-variable-driven configuration for node ID, bind address, data directory, election timeout, and heartbeat timeout.
Local Testing: Successfully ran a 3-node cluster on localhost. Verified write replication, all three consistency modes, transparent write forwarding, delete operations, and leader failover with automatic re-election.

AI Cost and Benefits
Benefits: Significantly accelerated the learning curve for hashicorp/raft. Helped identify subtle bugs (e.g., request body consumed before write forwarding causing EOF errors, LeaderLeaseTimeout misconfigured larger than HeartbeatTimeout) during code review. Provided architectural guidance on consistency mode design.
Costs: Required careful validation. AI suggestions occasionally needed adjustment. All code was reviewed and understood by the team before use.

4. Objectives
Short-Term (Course Timeline)

Deploy 5-node Raft cluster on AWS EC2 with automated provisioning
Execute all three chaos experiments and collect quantitative data
Produce charts and analysis demonstrating fault tolerance, consistency guarantees, and read scalability
Deliver a complete final report with reproducible experiment methodology

Long-Term (Beyond Course)

Production hardening: request retry with backoff, connection pooling
Enhanced observability: Prometheus metrics export and Grafana dashboards for live monitoring of Raft state, commit lag, and request latency
Dynamic membership: support adding/removing nodes at runtime via Raft configuration changes
Multi-region deployment: cross-region replication with region-aware read routing
Open-source toolkit: package the chaos engineering scripts as a reusable toolkit for testing any Raft-based system


5. Related Work
Course Readings

Raft Paper (Ongaro & Ousterhout, 2014): Our system directly implements the Raft consensus protocol via hashicorp/raft, following the paper's design for leader election, log replication, and safety.
Dynamo Paper (DeCandia et al., 2007): Dynamo uses eventual consistency with vector clocks; our system provides stronger guarantees. Our stale read mode is conceptually similar to Dynamo's eventual consistency, but our default and strong modes offer bounded and linearizable consistency.
CAP Theorem (Brewer, 2000): Our three consistency modes explicitly let clients choose their position on the CAP spectrum — strong reads sacrifice availability for consistency, stale reads sacrifice consistency for availability.

Related Piazza Projects

Sharded Distributed KV Store (https://piazza.com/class/mk3hftotl6e229/post/1124): Most architecturally similar. Both use hashicorp/raft in Go. Their project adds consistent-hashing-based sharding across multiple Raft groups; ours uses a single 5-node cluster and focuses on chaos engineering depth and correctness verification under failure.
Fault-Tolerant Distributed Rate Limiter (https://piazza.com/class/mk3hftotl6e229/post/1119): Also uses hashicorp/raft in Go, but applies Raft to replicate token bucket state. Their unique contribution is a Raft vs. Redis latency comparison; ours is systematic chaos engineering across three failure modes with post-healing correctness verification.
High-Availability L7 Load Balancer (https://piazza.com/class/mk3hftotl6e229/post/957): Least overlapping. They use Redis pub/sub for distributed state on ECS Fargate. Both their Experiment 3 and our Experiment 3 ask the same question — does adding nodes help throughput, or does shared state become the bottleneck? Key difference: they use an eventually-consistent managed service; we implement the consistency protocol ourselves.


6. Methodology
System Architecture
Our system consists of 5 identical Go nodes, each running:

Raft consensus layer (hashicorp/raft) — leader election, log replication, snapshotting
FSM — mutex-protected in-memory map that applies committed log entries
HTTP API (Gin framework) — client-facing REST endpoints for KV operations

Client Request → HTTP API → Raft Leader? → raft.Apply() → Log Replication → FSM Apply → Response
                                ↓ No
                        Forward to leader (transparent proxy)
Read Consistency Modes
ModeBehaviorTradeoffstrongLeader only + Barrier()Linearizable, all reads hit one nodedefaultAny node, reject if LastAppliedAt > 500ms oldBounded staleness with horizontal scalingstaleAny node, immediate readLowest latency, may return stale data
Chaos Engineering Approach

Leader kill: Kill the leader process via SSH. Measure time to new election and client-visible downtime.
Network partition: Use iptables rules on EC2 to isolate node subsets. Verify majority continues, minority halts, state converges after healing.
Latency injection: Use tc (traffic control) to add artificial delay. Measure impact on commit latency and throughput.

Experiments in Detail
Experiment 1: Leader Crash Recovery

5-node cluster under 200 writes/sec; kill leader at t=30s
Test with election timeouts: 150ms, 300ms, 500ms; also back-to-back kills
Metrics: election time, client downtime, writes lost, throughput recovery curve

Experiment 2: Network Partition Behavior

Three scenarios: minority isolation (2 nodes), leader isolation, symmetric 2-2-1 split
After healing: correctness checker compares all 5 nodes' state
Metrics: throughput during partition, write rejection rate, reconciliation time

Experiment 3: Read Scalability

Phase A: 90% read / 10% write at 2,000 req/sec — compare leader-only vs. follower reads with 3, 4, 5 nodes
Phase B: 5,000 req/sec with 5 nodes — compare strong, default, stale modes
Metrics: throughput scaling factor, latency p50/p95/p99, measured staleness

AI Usage in Methodology
Claude Code is used as a learning tool for hashicorp/raft API guidance, code review, experiment scripts and analysis code. All AI-assisted code is reviewed and validated before integration.

7. Preliminary Results
TestResultValidated PropertyWrite to leaderPassedLog replication works correctlyStrong read on leaderPassedLinearizable read enforcementDefault read — stale node correctly rejectedPassedBounded staleness enforcementStale read on followerPassedImmediate local readDelete keyPassedDELETE operation through Raft consensusTransparent write forwarding from followerPassedFollower proxies writes to leaderLeader failoverPassed (new leader elected, term 2→4)Automatic election and recoveryPre-failover data survivalPassedRaft safety property holds
See Part II above for detailed curl output.
What Remains

Experiment 1 (Leader Crash Recovery): Need EC2 deployment and load generator to measure election time and throughput recovery quantitatively. Local failover confirms the mechanism works.
Experiment 2 (Network Partition): Requires iptables-based partition injection on EC2. Cannot be fully tested locally.
Experiment 3 (Read Scalability): Need load generator and EC2 deployment to measure throughput scaling and latency percentiles across consistency modes.

Anticipated Worst-Case Workload
The pathological worst case is a symmetric network partition (2-2-1) combined with high write load. No partition has majority, so all writes are rejected cluster-wide. Clients retrying against a minority partition will see 100% failure with increasing latency as Raft election attempts time out. This is the expected and correct behavior — Raft sacrifices availability to preserve consistency — but it represents the worst client experience.

8. Impact
Distributed consensus is the backbone of modern infrastructure — Kubernetes (etcd), HashiCorp (Consul/Nomad), and CockroachDB all rely on Raft. Our project provides:

A clear, minimal Raft KV implementation others can study to understand consensus internals without the complexity of production systems like etcd
Reproducible chaos experiments with quantitative results showing exactly how Raft handles leader crashes, network partitions, and degraded networks
Empirical data on the consistency-throughput tradeoff — a question every distributed system designer faces

We welcome classmates to test our system once deployed on EC2. The HTTP API is simple (just curl), and we can provide a public endpoint for read experiments.
