# Archimedes

**A Physics-Based Simulator for Distributed Systems Design**

Archimedes is an interactive playground that treats system architecture as a physics problem. Instead of static diagrams, you build a live system where requests flow through a topology of blocks. As you increase the load, you watch friction (latency) and pressure (resource exhaustion) degrade your architecture from healthy green to failing red.

Think [TensorFlow Playground](https://playground.tensorflow.org/), but for infrastructure. Drag blocks onto a canvas, wire them together, crank up the RPS, and build intuition for back-of-envelope estimation and system design tradeoffs.

![Archimedes demo](demo.gif)

## Features

- **14 block types** — User, CDN, Load Balancer, API Gateway, Service, Worker, Analytics, Redis, SQL, KV Store, Document DB, Elasticsearch, Kafka, Object Storage
- **Physics-based simulation** — capacity, contention, queue depth, and drop counting
- **Stateful Ticker behaviors** — connection pools, memory eviction, cache hit ratios, segment merges, page cache pressure, partition hotspots, and more
- **Read/write cost asymmetry** — each block models different costs for reads vs. writes
- **Real-time visualization** — 100ms tick loop streamed via SSE with per-block gauges and queue bars
- **Drag-and-drop canvas** — add blocks, wire them together, delete with a click
- **Pause and drain** — pause the simulation, drain queues, resume

## Getting Started

Requires [Go 1.23+](https://go.dev/dl/).

```bash
go run ./cmd/server
```

Open [http://localhost:8080](http://localhost:8080).

```bash
go test ./...
```

## Tech Stack

Go (stdlib only, zero external dependencies), HTMX, Tailwind CSS.

## Status

Early-stage and actively developed. All 14 block types have physics-based Ticker behaviors. Netflix preset topology included for quick demos.

## Roadmap

- [ ] Edge fan-out multipliers for broadcast patterns
- [ ] Export/import topologies as JSON
- [ ] More preset scenarios (e.g. "design for Black Friday")
- [ ] Cost modeling overlay
- [ ] Latency percentile tracking (p50/p95/p99)

## Contributing

Contributions welcome — open an issue or submit a pull request.

## License

[MIT](LICENSE)
