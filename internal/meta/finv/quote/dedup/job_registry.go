package dedup

import (
	"context"
	"fmt"
	"sync"
)

// JobExecutor 独立 job 子包注册的执行入口。
type JobExecutor func(context.Context, *Service, *Result) error

var (
	jobExecutorsMu sync.RWMutex
	jobExecutors   = map[string]JobExecutor{}
)

// RegisterJobExecutor 由 job 子包在 init 中注册执行器。
func RegisterJobExecutor(kind string, executor JobExecutor) {
	jobExecutorsMu.Lock()
	defer jobExecutorsMu.Unlock()
	jobExecutors[kind] = executor
}

func lookupJobExecutor(kind string) (JobExecutor, bool) {
	jobExecutorsMu.RLock()
	defer jobExecutorsMu.RUnlock()
	executor, ok := jobExecutors[kind]
	return executor, ok
}

func (s *Service) dispatch(ctx context.Context, kind string, result *Result) error {
	if executor, ok := lookupJobExecutor(kind); ok {
		return executor(ctx, s, result)
	}
	return fmt.Errorf("未知 Job 或执行器未注册: %s", kind)
}
