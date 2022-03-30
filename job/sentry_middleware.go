// Copyright (c) The RAI Authors

package job

import (
	"fmt"
	"github.com/retail-ai-inc/bean"
	"github.com/retail-ai-inc/bean/options"

	"github.com/getsentry/sentry-go"
	"github.com/gocraft/work"
)

func NewSentryJobMiddlewareJob(b *bean.Bean) JobMiddleware {

	return func(next JobHandler) JobHandler {
		return func(job *work.Job) error {

			defer func() {

				if r := recover(); r != nil {
					if !options.SentryOn {
						b.Echo.Logger.Error(r)
						return
					}

					err, ok := r.(error)
					if ok {
						sentry.CaptureException(err)
					} else {
						sentry.CaptureMessage(fmt.Sprintf("%+v", r))
					}
				}
			}()

			return next(job)
		}
	}
}
