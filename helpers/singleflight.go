package helpers

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sync/singleflight"
)

type callback func() (interface{}, error)

var (
	singleFlightGroup = new(singleflight.Group)
	waitTime          = time.Duration(100) * time.Millisecond
	maxWaitTime       = time.Duration(2000) * time.Millisecond
)

func SingleDoChan[T any](ctx context.Context, key string, call callback, retry int, ttl ...time.Duration) (data T, err error) {
	result := singleFlightGroup.DoChan(key, func() (result interface{}, err error) {
		defer func() {
			if e := recover(); e != nil {
				err = errors.WithStack(fmt.Errorf("%v", e))
			}
		}()

		if len(ttl) > 0 {
			forgetTimer := time.AfterFunc(ttl[0], func() {
				singleFlightGroup.Forget(key)
			})
			defer forgetTimer.Stop()
		}

		for i := 0; i <= retry; i++ {
			result, err = call()
			if err == nil {
				return result, nil
			}
			if i == retry {
				return nil, err
			}

			waitTime := JitterBackoff(waitTime, maxWaitTime, i)
			select {
			case <-time.After(waitTime):
			case <-ctx.Done():
				return nil, errors.WithStack(ctx.Err())
			}
		}
		return nil, err
	})

	select {
	case r := <-result:
		if r.Err != nil {
			err = r.Err
			return
		}
		val, ok := r.Val.(T)
		if !ok {
			return
		}
		return val, nil
	case <-ctx.Done():
		if ctx.Err() != nil {
			err = ctx.Err()
			return
		}
	}
	return
}
