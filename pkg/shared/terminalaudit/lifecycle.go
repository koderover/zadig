package terminalaudit

import (
	"context"
	"sync"
)

var (
	// Process lifecycle state is kept separately from per-session registry state.
	processContextMu sync.RWMutex
	processContext   = context.Background()
)

// SetProcessContext updates the parent context used by active terminal sessions.
func SetProcessContext(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	processContextMu.Lock()
	processContext = ctx
	processContextMu.Unlock()
}

func processLifecycleContext() context.Context {
	processContextMu.RLock()
	defer processContextMu.RUnlock()
	return processContext
}
