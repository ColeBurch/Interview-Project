package main

import (
	"sort"
	"sync"
)

type viewer struct {
	name    string
	updates chan []string
}
type presence struct {
	mu      sync.Mutex
	reports map[string]map[*viewer]struct{}
}

func newPresence() *presence {
	return &presence{reports: make(map[string]map[*viewer]struct{})}
}

func (p *presence) join(reportID, name string) (*viewer, func()) {
	v := &viewer{name: name, updates: make(chan []string, 8)}

	p.mu.Lock()
	if p.reports[reportID] == nil {
		p.reports[reportID] = make(map[*viewer]struct{})
	}
	p.reports[reportID][v] = struct{}{}
	p.broadcastLocked(reportID)
	p.mu.Unlock()

	leave := func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		delete(p.reports[reportID], v)
		if len(p.reports[reportID]) == 0 {
			delete(p.reports, reportID)
			return
		}
		p.broadcastLocked(reportID)
	}
	return v, leave
}

func (p *presence) broadcastLocked(reportID string) {
	names := p.namesLocked(reportID)
	for v := range p.reports[reportID] {
		select {
		case v.updates <- names:
		default:
		}
	}
}

func (p *presence) namesLocked(reportID string) []string {
	seen := make(map[string]struct{})
	names := make([]string, 0, len(p.reports[reportID]))
	for v := range p.reports[reportID] {
		if _, ok := seen[v.name]; ok {
			continue
		}
		seen[v.name] = struct{}{}
		names = append(names, v.name)
	}
	sort.Strings(names)
	return names
}
