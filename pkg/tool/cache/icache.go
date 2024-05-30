// Forked from github.com/k8sgpt-ai/k8sgpt
// Some parts of this file have been modified to make it functional in Zadig

package cache

import "github.com/koderover/zadig/v2/pkg/config"

type ICache interface {
	Store(key string, data string) error
	Load(key string) (string, error)
	List() ([]string, error)
	Exists(key string) bool
	IsCacheDisabled() bool
}

type CacheType string

var (
	CacheTypeRedis CacheType = "redis"
	CacheTypeMem   CacheType = "memory"
)

// New returns a memory cache which implements the iCache interface
func New(noCache bool, cacheType CacheType) ICache {
	switch cacheType {
	case CacheTypeRedis:
		return NewRedisCacheAI(config.RedisCommonCacheTokenDB(), noCache)
	case CacheTypeMem:
		return NewMemCache(noCache)
	default:
		return NewMemCache(noCache)
	}
}
