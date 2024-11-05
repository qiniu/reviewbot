package cache

import (
	"sync"
	"time"
)

type IssueCache struct {
	mu      sync.RWMutex
	data    map[string]string
	ttl     time.Duration
	lastGet time.Time
}

func NewIssueReferencesCache(ttl time.Duration) *IssueCache {
	return &IssueCache{
		data: make(map[string]string),
		ttl:  ttl,
	}
}

func (c *IssueCache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	issue, isFound := c.data[key]
	return issue, isFound
}

func (c *IssueCache) Set(key string, issueContent string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = issueContent
	c.lastGet = time.Now()
}

func (c *IssueCache) IsExpired() bool {
	return time.Since(c.lastGet) > c.ttl
}
