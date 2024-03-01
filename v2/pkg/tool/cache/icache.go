// Forked from github.com/k8sgpt-ai/k8sgpt
// Some parts of this file have been modified to make it functional in Zadig

package cache

type ICache interface {
	Store(key string, data string) error
	Load(key string) (string, error)
	List() ([]string, error)
	Exists(key string) bool
	IsCacheDisabled() bool
}

// New returns a memory cache which implements the iCache interface
func New(noCache bool) ICache {
	return NewMemCache(noCache)
}
