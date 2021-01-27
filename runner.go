package grace

import (
	"context"
)

// Runner is something that can be run and gracefully stopped.
type Runner interface {
	// Run starts the Runner and blocks until it's done.
	Run(ctx context.Context) error
}

// RunnerFunc turns a basic func into a Runner.
func RunnerFunc(runFn func(ctx context.Context) error) Runner {
	return &funcRunner{
		runFn: runFn,
	}
}

type funcRunner struct {
	runFn func(ctx context.Context) error
}

// Run starts the Runner and blocks until it's done.
func (fr *funcRunner) Run(ctx context.Context) error {
	return fr.runFn(ctx)
}
