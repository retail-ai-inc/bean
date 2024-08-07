package helpers

import (
	"context"
	"errors"
	"fmt"
	"time"

	pkgerrors "github.com/pkg/errors"
	"golang.org/x/sync/singleflight"
)

var (
	singleFlightGroup = new(singleflight.Group)
	waitTime          = time.Duration(100) * time.Millisecond
	maxWaitTime       = time.Duration(2000) * time.Millisecond
)

func SingleDo[T any](ctx context.Context, key string, call func() (T, error), retry uint, ttl ...time.Duration) (data T, err error) {
	result, e, _ := singleFlightGroup.Do(key, func() (result interface{}, err error) {
		if len(ttl) > 0 {
			forgetTimer := time.AfterFunc(ttl[0], func() {
				singleFlightGroup.Forget(key)
			})
			defer forgetTimer.Stop()
		}

		timer := time.NewTimer(waitTime)
		defer timer.Stop()

		for i := 0; i <= int(retry); i++ {
			result, err = call()
			if err == nil {
				return result, nil
			}
			if i == int(retry) {
				return nil, err
			}

			timer.Reset(JitterBackoff(waitTime, maxWaitTime, i))
			select {
			case <-timer.C:
			case <-ctx.Done():
				return nil, errors.Join(ctx.Err(), context.Cause(ctx), err)
			}
		}
		return nil, err
	})

	if e != nil {
		err = e
		return
	}

	val, ok := result.(T)
	if !ok {
		err = pkgerrors.WithStack(fmt.Errorf("expected type %T but got type %T", data, result))
		return
	}
	return val, nil
}

func SingleDoChan[T any](ctx context.Context, key string, call func() (T, error), retry uint, ttl ...time.Duration) (data T, err error) {
	result := singleFlightGroup.DoChan(key, func() (result interface{}, err error) {
		defer func() {
			if e := recover(); e != nil {
				err = pkgerrors.WithStack(fmt.Errorf("%v", e))
			}
		}()

		if len(ttl) > 0 {
			forgetTimer := time.AfterFunc(ttl[0], func() {
				singleFlightGroup.Forget(key)
			})
			defer forgetTimer.Stop()
		}

		timer := time.NewTimer(waitTime)
		defer timer.Stop()

		for i := 0; i <= int(retry); i++ {
			result, err = call()
			if err == nil {
				return result, nil
			}
			if i == int(retry) {
				return nil, err
			}

			timer.Reset(JitterBackoff(waitTime, maxWaitTime, i))
			select {
			case <-timer.C:
			case <-ctx.Done():
				return nil, err
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
			err = pkgerrors.WithStack(fmt.Errorf("expected type %T but got type %T", data, r.Val))
			return
		}
		return val, nil
	case <-ctx.Done():
		err = pkgerrors.WithStack(errors.Join(ctx.Err(), context.Cause(ctx)))
		return
	}
}
