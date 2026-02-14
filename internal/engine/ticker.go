package engine

import (
	"sync"
	"time"
)

type TickResult struct {
	Tick   int           `json:"tick"`
	Blocks []BlockResult `json:"blocks"`
}

type Sim struct {
	mu      sync.Mutex
	graph   *Graph
	rps     float64
	tick    int
	stop    chan struct{}
	running bool
	subs    []chan TickResult
}

func NewSim() *Sim {
	return &Sim{}
}

func (s *Sim) Subscribe() chan TickResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan TickResult, 1)
	s.subs = append(s.subs, ch)
	return ch
}

func (s *Sim) Unsubscribe(ch chan TickResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, sub := range s.subs {
		if sub == ch {
			s.subs = append(s.subs[:i], s.subs[i+1:]...)
			close(ch)
			return
		}
	}
}

func (s *Sim) Play(topo Topology) error {
	g, err := BuildGraph(topo)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		s.stopLocked()
	}

	s.graph = g
	s.rps = topo.RPS
	s.tick = 0
	s.stop = make(chan struct{})
	s.running = true

	go s.loop()
	return nil
}

func (s *Sim) Pause() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		s.stopLocked()
	}
}

func (s *Sim) stopLocked() {
	close(s.stop)
	s.running = false
}

func (s *Sim) UpdateRPS(rps float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rps = rps
}

func (s *Sim) loop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.mu.Lock()
			s.tick++
			results, err := Simulate(s.graph, s.rps)
			if err != nil {
				s.mu.Unlock()
				continue
			}
			tr := TickResult{Tick: s.tick, Blocks: results}
			for _, ch := range s.subs {
				select {
				case ch <- tr:
				default: // slow consumer, drop
				}
			}
			s.mu.Unlock()
		}
	}
}
