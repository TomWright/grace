package grace

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"
)

var (
	// ErrShutdownSignalReceived is used when a shutdown signal is received.
	// It will cause a graceful shutdown.
	ErrShutdownSignalReceived = errors.New("shutdown signal received")

	// ErrShutdownRequestReceived is used when a shutdown is initiated without a signal.
	// It will cause a graceful shutdown.
	ErrShutdownRequestReceived = errors.New("shutdown request received")

	// ErrImmediateShutdownSignalReceived is used when a shutdown signal is received for the second time.
	// It will cause an immediate shutdown.
	ErrImmediateShutdownSignalReceived = errors.New("immediate shutdown signal received")
)

// RecoveredPanicError represents a recovered panic.
type RecoveredPanicError struct {
	// Err is the recovered error.
	Err interface{}
	// Stack is the stack trace from the recover.
	Stack []byte
}

// Error returns the recovered panic as a string
func (p RecoveredPanicError) Error() string {
	return fmt.Sprintf("%v", p.Err)
}

// Init returns a new and initialised instance of Grace.
func Init(ctx context.Context, options ...InitOption) *Grace {
	ctx, cancel := context.WithCancel(ctx)
	g := &Grace{
		wg:       &sync.WaitGroup{},
		ctx:      ctx,
		cancelFn: cancel,
		errCh:    make(chan error),
		LogFn:    log.Printf,
	}
	g.ErrHandler = DefaultErrorHandler(g)

	for _, option := range options {
		option.Apply(g)
	}

	go g.handleErrors()
	go g.handleShutdownSignals()
	return g
}

// Grace manages Runners and handles errors.
type Grace struct {
	// ErrHandler is the active error handler grace will pass errors to.
	ErrHandler ErrHandlerFn
	// LogFn is the log function that grace will use.
	LogFn func(format string, v ...interface{})

	wg       *sync.WaitGroup
	ctx      context.Context
	cancelFn context.CancelFunc
	errCh    chan error
}

// ErrHandlerFn is an error handler used to handle errors returned from Runners.
// Returns true if a graceful shutdown should be triggered as a result of the error.
type ErrHandlerFn func(err error) bool

// Context returns the grace context.
// Use this throughout your application to be notified when a graceful shutdown has started.
func (l *Grace) Context() context.Context {
	return l.ctx
}

// Wait will block until all Runners have stopped running.
func (l *Grace) Wait() {
	l.wg.Wait()
}

// Run runs a new Runner.
func (l *Grace) Run(runner Runner) {
	l.wg.Add(1)
	go l.run(runner)
}

// runRunner executes the given runner and returns any panics as a RecoveredPanicError.
// If a runner panics it will cause a graceful shutdown.
func (l *Grace) runRunner(ctx context.Context, runner Runner) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = RecoveredPanicError{Err: r, Stack: debug.Stack()}
		}
	}()
	err = nil
	err = runner.Run(ctx)
	return
}

func (l *Grace) run(runner Runner) {
	defer l.wg.Done()

	runErrs := make(chan error)
	runWg := &sync.WaitGroup{}

	runWg.Add(1)
	go func() {
		defer func() {
			runWg.Done()
		}()
		err := l.runRunner(l.ctx, runner)
		if err != nil {
			// If the context is already done, we won't be able to write to runErrs
			// as nothing is waiting to read from it anymore.
			// We may want to handle this slightly differently to ensure all errors are handled, but this can
			// be safely ignored for now.
			select {
			case runErrs <- err:
			default:
			}
		}
	}()

	select {
	case runErr := <-runErrs:
		// The runner returned an error.
		l.errCh <- runErr
	case <-l.ctx.Done():
		// We've been told to shutdown.
		// The runner should shutdown on ctx.Done() so we can just wait for it to finish.
		runWg.Wait()
	}
}

func (l *Grace) handleShutdownSignals() {
	signals := make(chan os.Signal, 1)

	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

	go func() {
		count := 0
		for sig := range signals {
			count++
			if count > 1 || sig == syscall.SIGKILL {
				l.errCh <- ErrImmediateShutdownSignalReceived
				continue
			}
			l.errCh <- ErrShutdownSignalReceived
		}
	}()
}

// DefaultErrorHandler returns a standard error handler that will log errors and trigger a shutdown.
func DefaultErrorHandler(g *Grace) func(err error) bool {
	return func(err error) bool {
		if err == ErrImmediateShutdownSignalReceived {
			os.Exit(1)
		}

		if g.LogFn != nil {
			g.LogFn("default error handler: %s\n", err.Error())
		}

		// Always shutdown on error.
		return true
	}
}

func (l *Grace) handleErrors() {
	go func() {
		for err := range l.errCh {
			if l.ErrHandler(err) {
				l.cancelFn()
			}
		}
	}()
}

// Shutdown will start a manual shutdown.
func (l *Grace) Shutdown() {
	l.errCh <- ErrShutdownRequestReceived
}

// InitOption allows you to modify the way grace is initialised.
type InitOption interface {
	// Apply allows you to apply options to the grace instance as it is being initialised.
	Apply(g *Grace)
}

type initOptionFn struct {
	apply func(g *Grace)
}

// Apply allows you to apply options to the grace instance as it is being initialised.
func (o *initOptionFn) Apply(g *Grace) {
	o.apply(g)
}

// InitOptionFn returns an InitOption that applies the given function.
func InitOptionFn(fn func(g *Grace)) InitOption {
	return &initOptionFn{
		apply: fn,
	}
}
