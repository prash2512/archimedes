# Archimedes

**A Physics-Based Simulator for Distributed Systems Design**

Archimedes is an interactive playground that treats system architecture as a physics problem. Instead of static diagrams, you build a State Machine where Request Agents navigate through a topology of nodes. As you increase the load, you can watch the "friction" (latency) and "pressure" (resource exhaustion) turn your architecture from a healthy green to a failing red.

Think [TensorFlow Playground](https://playground.tensorflow.org/), but for infrastructure. Drag blocks onto a canvas, wire them together, crank up the RPS, and build intuition for back-of-envelope estimation and system design tradeoffs.

## Running

```bash
go run ./cmd/server
```

Open `http://localhost:8080`.

## Building

```bash
go build ./cmd/server
```

## Tests

```bash
go test ./...
```

## Tech stack

Go, HTMX, Tailwind CSS.

## Status

Early development. The canvas with drag-and-drop blocks is working. Connections, simulation engine, and real-time metrics are next.
