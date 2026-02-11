package chain

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

type MultiRPCClient struct {
	clients       []*RPCClient
	index         int
	failCount     int
	failThreshold int
	mu            sync.Mutex
}

func NewMultiRPCClient(endpoints []string, failThreshold int) (*MultiRPCClient, error) {
	list := sanitizeEndpoints(endpoints)
	if len(list) == 0 {
		return nil, errors.New("rpc endpoints is empty")
	}
	if failThreshold <= 0 {
		failThreshold = 3
	}
	clients := make([]*RPCClient, 0, len(list))
	for _, ep := range list {
		clients = append(clients, NewRPCClient(ep))
	}
	return &MultiRPCClient{
		clients:       clients,
		index:         0,
		failCount:     0,
		failThreshold: failThreshold,
	}, nil
}

func (m *MultiRPCClient) BaseURL() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.clients[m.index].baseURL
}

func (m *MultiRPCClient) LatestHeight(ctx context.Context) (int64, error) {
	m.mu.Lock()
	start := m.index
	m.mu.Unlock()

	var lastErr error
	for attempts := 0; attempts < len(m.clients); attempts++ {
		client, idx := m.currentClient()
		out, err := client.LatestHeight(ctx)
		if err == nil {
			m.resetFailures(idx)
			return out, nil
		}
		lastErr = err
		m.noteFailure(idx)
		if m.shouldRotate() || len(m.clients) > 1 {
			m.rotate()
		}
		if idx == start && attempts > 0 {
			break
		}
	}
	return 0, lastErr
}

func (m *MultiRPCClient) TxSearch(ctx context.Context, query string, page, perPage int) (*TxSearchResult, error) {
	m.mu.Lock()
	start := m.index
	m.mu.Unlock()

	var lastErr error
	for attempts := 0; attempts < len(m.clients); attempts++ {
		client, idx := m.currentClient()
		out, err := client.TxSearch(ctx, query, page, perPage)
		if err == nil {
			m.resetFailures(idx)
			return out, nil
		}
		lastErr = err
		m.noteFailure(idx)
		if m.shouldRotate() || len(m.clients) > 1 {
			m.rotate()
		}
		if idx == start && attempts > 0 {
			break
		}
	}
	return nil, lastErr
}

func (m *MultiRPCClient) TxByHash(ctx context.Context, hash string) (*Tx, error) {
	m.mu.Lock()
	start := m.index
	m.mu.Unlock()

	var lastErr error
	for attempts := 0; attempts < len(m.clients); attempts++ {
		client, idx := m.currentClient()
		out, err := client.TxByHash(ctx, hash)
		if err == nil {
			m.resetFailures(idx)
			return out, nil
		}
		lastErr = err
		m.noteFailure(idx)
		if m.shouldRotate() || len(m.clients) > 1 {
			m.rotate()
		}
		if idx == start && attempts > 0 {
			break
		}
	}
	return nil, lastErr
}

func (m *MultiRPCClient) BlockTime(ctx context.Context, height int64) (time.Time, error) {
	m.mu.Lock()
	start := m.index
	m.mu.Unlock()

	var lastErr error
	for attempts := 0; attempts < len(m.clients); attempts++ {
		client, idx := m.currentClient()
		out, err := client.BlockTime(ctx, height)
		if err == nil {
			m.resetFailures(idx)
			return out, nil
		}
		lastErr = err
		m.noteFailure(idx)
		if m.shouldRotate() || len(m.clients) > 1 {
			m.rotate()
		}
		if idx == start && attempts > 0 {
			break
		}
	}
	return time.Time{}, lastErr
}

func (m *MultiRPCClient) currentClient() (*RPCClient, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.clients[m.index], m.index
}

func (m *MultiRPCClient) resetFailures(idx int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.index == idx {
		m.failCount = 0
	}
}

func (m *MultiRPCClient) noteFailure(idx int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.index == idx {
		m.failCount++
	}
}

func (m *MultiRPCClient) shouldRotate() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.failCount >= m.failThreshold
}

func (m *MultiRPCClient) rotate() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.index = (m.index + 1) % len(m.clients)
	m.failCount = 0
}

func sanitizeEndpoints(endpoints []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(endpoints))
	for _, ep := range endpoints {
		ep = strings.TrimSpace(ep)
		if ep == "" {
			continue
		}
		ep = strings.TrimRight(ep, "/")
		if _, ok := seen[ep]; ok {
			continue
		}
		seen[ep] = struct{}{}
		out = append(out, ep)
	}
	return out
}
