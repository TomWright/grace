package grace

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
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

// Init returns a new and initialised instance of Grace.
func Init(ctx context.Context) *Grace {
	ctx, cancel := context.WithCancel(ctx)
	g := &Grace{
		wg:         &sync.WaitGroup{},
		ctx:        ctx,
		cancelFn:   cancel,
		errCh:      make(chan error),
		ErrHandler: DefaultErrorHandler,
	}
	go g.handleErrors()
	go g.handleShutdownSignals()
	return g
}

// Grace manages Runners and handles errors.
type Grace struct {
	// ErrHandler is the active error handler grace will pass errors to.
	ErrHandler ErrHandlerFn

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

func (l *Grace) run(runner Runner) {
	defer l.wg.Done()

	runErrs := make(chan error)
	runWg := &sync.WaitGroup{}

	runWg.Add(1)
	go func() {
		defer runWg.Done()
		err := runner.Run(l.ctx)
		if err != nil {
			runErrs <- err
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

// DefaultErrorHandler is a standard error handler that will log errors and trigger a shutdown.
func DefaultErrorHandler(err error) bool {
	if err == ErrImmediateShutdownSignalReceived {
		os.Exit(1)
	}

	log.Printf("default error handler: %s", err.Error())

	// Always shutdown on error.
	return true
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
