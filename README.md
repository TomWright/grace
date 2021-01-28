[![Go Report Card](https://goreportcard.com/badge/github.com/TomWright/grace)](https://goreportcard.com/report/github.com/TomWright/grace)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/tomwright/grace)](https://pkg.go.dev/github.com/tomwright/grace)
![Test](https://github.com/TomWright/grace/workflows/Test/badge.svg)
[![codecov](https://codecov.io/gh/TomWright/grace/branch/main/graph/badge.svg)](https://codecov.io/gh/TomWright/grace)
![GitHub License](https://img.shields.io/github/license/TomWright/grace)
![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/TomWright/grace?label=latest%20release)

# grace

Easy graceful shutdowns in your go applications.

## Table of Contents

- [Usage](#usage)
- [Quickstart](#quickstart)
- [Runners](#runners)
  - [Pre-made Runners](#pre-made-runners)
- [Triggering Shutdowns](#triggering-shutdowns)

## Usage

First import the module:

```bash
go get github.com/tomwright/grace
```

## Quickstart

```go
package main

import (
    "context"
    "github.com/tomwright/grace"
    "time"
)

type myRunner struct {
}

func (mr *myRunner) Run(ctx context.Context) error {
    // Block until we are told to shutdown or until all work is done.
    <-ctx.Done()
    return nil
}

func main() {
    // Initialise grace.
    g := grace.Init(context.Background())

    // The following runners are equivalent.
    runnerA := &myRunner{}
    runnerB := grace.RunnerFunc(func(ctx context.Context) error {
        // Block until we are told to shutdown or until all work is done.
        <-ctx.Done()
        return nil
    })
	
    // Run both runners.
    g.Run(runnerA)
    g.Run(runnerB)

    go func() {
        // Manually shutdown after 2.5 seconds.
        // Alternatively leave this out and rely on os signals to shutdown.
        <-time.After(time.Millisecond * 2500)
        g.Shutdown()
    }()

    // Optionally wait for the shutdown signal to be sent.
    <-g.Context().Done()

    // Wait for all Runners to end.
    g.Wait()
}
```

## Runners

Grace implements logic to execute and gracefully shutdown any number of `Runner`s.

Each runner must react when the given contexts `Done` channel is closed.

A runner must implement the `Runner` interface. It can do this by implementing the `Run(context.Context) error` func, or by passing a func to `RunnerFunc()`.

### Pre-made Runners

Grace doesn't provide any runners in this module in order to reduce forced dependencies... instead please import the required modules below.

Implemented your own runner that you want to share? Raise a PR!

- [HTTPServer](https://github.com/TomWright/gracehttpserverrunner)
- [GRPCServer](https://github.com/TomWright/gracegrpcserverrunner)

## Triggering Shutdowns

Shutdowns can be triggered in a number of ways:

1. `SIGKILL` will immediately terminate the application using `os.Exit(1)`.
2. `SIGINT` or `SIGTERM` will trigger a graceful shutdown. Sending 2 or more of these will immediately exit the application using `os.Exit(1)`.
3. Calling `Shutdown()` on a grace instance will start a graceful shutdown.
4. Cancelling the context you gave to grace will start a graceful shutdown.
