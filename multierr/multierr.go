// Package multierr provides a mechanism for dealing with concurrent
// errors.
package multierr

import (
	"context"
	"sync"
)

// MultiErr is a concurrency structure for handling the potential of
// multiple concurrently produced errors.
type MultiErr struct {
	wg     sync.WaitGroup
	cancel context.CancelFunc

	errs []error
	merr sync.Mutex
}

// WithContext creates a new MultiErr that uses a given
// context.Context. It returns both the new MultiErr and a new child
// context that is canceled when the MultiErr is finished, either due
// to an error or not.
func WithContext(ctx context.Context) (*MultiErr, context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	return &MultiErr{
		cancel: cancel,
	}, ctx
}

// Go starts a function concurrently. If the function returns an
// error, the MultiErr is canceled and the error is added to the list
// of returned arrors.
func (me *MultiErr) Go(f func() error) {
	me.wg.Add(1)
	go func() {
		defer me.wg.Done()

		err := f()
		if err != nil {
			me.merr.Lock()
			me.errs = append(me.errs, err)
			me.merr.Unlock()

			me.cancel()
		}
	}()
}

// Wait waits for all of the functions started with Go to finish, then
// cancels the context returned from WithContext and returns all of
// the errors that were returned from those functions in an undefined
// order.
func (me *MultiErr) Wait() (errs []error) {
	defer me.cancel()

	me.wg.Wait()

	me.merr.Lock()
	errs = me.errs
	me.errs = nil
	me.merr.Unlock()

	return errs
}
