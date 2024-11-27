package cache

import (
	"sync"
	"time"
)

// DefaultTokenCache uses to cache provider impersonation tokens.
var DefaultTokenCache = NewGitHubAppTokenCache()

// TokenCache implements the cache for provider impersonation tokens.
type TokenCache struct {
	sync.RWMutex
	tokens map[string]tokenWithExp
}

type tokenWithExp struct {
	token string
	exp   time.Time
}

// NewGitHubAppTokenCache creates a new token cache.
func NewGitHubAppTokenCache() *TokenCache {
	return &TokenCache{
		tokens: make(map[string]tokenWithExp),
	}
}

func (c *TokenCache) GetToken(key string) (string, bool) {
	c.RLock()
	t, exists := c.tokens[key]
	c.RUnlock()

	if exists && t.exp.After(time.Now()) {
		return t.token, true
	}

	return "", false
}

func (c *TokenCache) SetToken(key string, token string, exp time.Time) {
	c.Lock()
	defer c.Unlock()
	c.tokens[key] = tokenWithExp{
		token: token,
		exp:   exp,
	}
}
