package terminalaudit

import (
	"context"
	"sync"
)

var (
	processContextMu sync.RWMutex
	processContext   = context.Background()
)

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
