// MIT License

// Copyright (c) The RAI Authors

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

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
