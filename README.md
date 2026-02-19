# Archimedes

**A Physics-Based Simulator for Distributed Systems Design**

Archimedes is an interactive playground that treats system architecture as a physics problem. Instead of static diagrams, you build a live system where requests flow through a topology of blocks. As you increase the load, you watch friction (latency) and pressure (resource exhaustion) degrade your architecture from healthy green to failing red.

Think [TensorFlow Playground](https://playground.tensorflow.org/), but for infrastructure. Drag blocks onto a canvas, wire them together, crank up the RPS, and build intuition for capacity planning and system design tradeoffs.

## Features

- **14 block types** — User, CDN, Load Balancer, API Gateway, Service, Worker, Analytics, Redis, SQL, KV Store, Document DB, Elasticsearch, Kafka, Object Storage
- **Physics-based simulation** — capacity, contention (quadratic degradation past 60%), bounded queues (max 5000), and drop counting
- **Stateful Ticker behaviors** — connection pools, LRU eviction, cache hit ratios, segment merges, page cache pressure, partition hotspots, bandwidth throttling, and more
- **Edge weights** — right-click any edge to set traffic percentage (e.g., 90% to Redis, 5% to Kafka). Presets ship with realistic weights
- **Cache absorption** — CDN and Redis actually reduce downstream traffic based on hit ratio, not just increase their own capacity
- **Per-block read/write ratios** — Redis sees 95% reads, Kafka sees 90% writes, SQL sees 70/30 — each block models its natural workload
- **Read/write cost asymmetry** — each block has different CPU, memory, and disk costs for reads vs. writes
- **Live config updates** — change replicas, shards, CPU cores, or edge weights during playback without restarting
- **Scaling** — replicas (horizontal), shards (data partitioning), CPU override per block via right-click config
- **Real-time visualization** — 100ms tick loop streamed via SSE with per-block gauges, queue bars, drop counters, and animated edges
- **Preset topologies** — Netflix and E-Commerce architectures with realistic edge weights, auto-loaded on first visit
- **Drag-and-drop canvas** — add blocks, wire them, delete with backspace, undo edges with Cmd+Z

## Getting Started

Requires [Go 1.23+](https://go.dev/dl/).

```bash
go run ./cmd/server
```

Open [http://localhost:8080](http://localhost:8080). The Netflix preset loads automatically.

```bash
go test ./...
```

## How It Works

1. **Build a topology** — drag blocks from the sidebar onto the canvas, click two blocks to connect them
2. **Set edge weights** — right-click an edge to control what fraction of traffic flows through it
3. **Configure blocks** — right-click a block to adjust replicas, shards, and CPU cores
4. **Crank the load** — use the RPS slider to increase traffic and the read/write slider to change the mix
5. **Watch it break** — blocks go green (healthy) to yellow (degraded) to red (failing). Queues build up, drops appear, edges change color

## Tech Stack

Go (stdlib only, zero external dependencies), HTMX, Tailwind CSS.

## License

[MIT](LICENSE)
