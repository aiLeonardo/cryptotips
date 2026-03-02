package engine

import (
	"context"
	"time"
)

type Runner struct {
	strategy Strategy
	deps     RuntimeDeps
}

func NewRunner(strategy Strategy, deps RuntimeDeps) *Runner {
	return &Runner{strategy: strategy, deps: deps}
}

func (r *Runner) RunOnce(ctx context.Context, strategyID, mode string) error {
	rctx := RuntimeContext{
		Context:    ctx,
		StrategyID: strategyID,
		Mode:       mode,
		Now:        time.Now().UTC(),
		Deps:       r.deps,
		StateStore: NewRedisStateStore(r.deps.Redis, strategyID),
	}
	return r.strategy.Run(rctx)
}

func (r *Runner) RunLoop(ctx context.Context, strategyID, mode string, interval time.Duration) error {
	if err := r.RunOnce(ctx, strategyID, mode); err != nil {
		return err
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := r.RunOnce(ctx, strategyID, mode); err != nil {
				r.deps.Logger.Errorf("strategy run error: %v", err)
			}
		}
	}
}

func (r *Runner) Status(ctx context.Context, strategyID, mode string) (map[string]any, error) {
	rctx := RuntimeContext{
		Context:    ctx,
		StrategyID: strategyID,
		Mode:       mode,
		Now:        time.Now().UTC(),
		Deps:       r.deps,
		StateStore: NewRedisStateStore(r.deps.Redis, strategyID),
	}
	return r.strategy.Status(rctx)
}
