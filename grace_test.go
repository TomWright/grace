package grace_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/tomwright/grace"
	"reflect"
	"testing"
	"time"
)

func ExampleInit() {
	g := grace.Init(context.Background())

	fmt.Println("Starting")

	g.Run(grace.RunnerFunc(func(ctx context.Context) error {
		for {
			select {
			case <-ctx.Done():
				fmt.Println("Graceful shutdown initiated")
				// Fake a 3 second shutdown
				for i := 3; i >= 0; i-- {
					fmt.Printf("Shutting down in %d\n", i)
					time.Sleep(time.Second * time.Duration(i))
				}
				return nil
			case <-time.After(time.Second):
				fmt.Println("Hello")
			}
		}
	}))

	go func() {
		<-time.After(time.Millisecond * 2500)
		fmt.Println("Triggering shutdown")
		g.Shutdown()
	}()

	fmt.Println("Started")
	<-g.Context().Done()

	g.Wait()
	fmt.Println("Shutdown")

	// Output:
	// Starting
	// Started
	// Hello
	// Hello
	// Triggering shutdown
	// Graceful shutdown initiated
	// Shutting down in 3
	// Shutting down in 2
	// Shutting down in 1
	// Shutting down in 0
	// Shutdown
}

func TestGrace_Run_Error(t *testing.T) {
	errExpected := errors.New("expected error")

	gotErrors := map[error]int{}
	g := grace.Init(context.Background())
	g.ErrHandler = func(err error) bool {
		if _, ok := gotErrors[err]; !ok {
			gotErrors[err] = 0
		}
		gotErrors[err]++
		return true
	}

	g.Run(grace.RunnerFunc(func(ctx context.Context) error {
		return errExpected
	}))

	select {
	case <-g.Context().Done():
	case <-time.After(time.Second):
		t.Errorf("did not send shutdown signal after 1s")
		g.Shutdown()
	}

	expErrors := map[error]int{
		errExpected: 1,
	}

	if !reflect.DeepEqual(expErrors, gotErrors) {
		t.Errorf("expected errors: %v, got errors: %v", expErrors, gotErrors)
		return
	}
}

func TestGrace_Run_Panic(t *testing.T) {
	found := false
	g := grace.Init(context.Background())
	g.ErrHandler = func(err error) bool {
		if e, ok := err.(grace.RecoveredPanicError); ok {
			if fmt.Sprintf("%v", e.Err) == "whoops" {
				found = true
			} else {
				t.Errorf("unexpected error: %T: %v", err, err)
			}
		} else {
			t.Errorf("unexpected error type: %T: %v", err, err)
		}
		return true
	}

	g.Run(grace.RunnerFunc(func(ctx context.Context) error {
		panic("whoops")
		return nil
	}))

	select {
	case <-g.Context().Done():
	case <-time.After(time.Second):
		t.Errorf("did not send shutdown signal after 1s")
		g.Shutdown()
	}

	if !found {
		t.Errorf("panic error not found")
	}
}
