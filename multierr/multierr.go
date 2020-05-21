package multierr

import (
	"context"
	"sync"
)

type MultiErr struct {
	wg     sync.WaitGroup
	cancel context.CancelFunc

	errs []error
	merr sync.Mutex
}

func WithContext(ctx context.Context) (*MultiErr, context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	return &MultiErr{
		cancel: cancel,
	}, ctx
}

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

func (me *MultiErr) Wait() (errs []error) {
	defer me.cancel()

	me.wg.Wait()

	me.merr.Lock()
	errs = me.errs
	me.errs = nil
	me.merr.Unlock()

	return errs
}
