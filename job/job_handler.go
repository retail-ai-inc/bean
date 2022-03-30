// Copyright (c) The RAI Authors

package job

import (
	"github.com/gocraft/work"
	"github.com/spf13/viper"
)

type JobHandler = func(job *work.Job) error
type JobMiddleware = func(next JobHandler) JobHandler

type Context struct {
	customerID int64
}

type Job struct {
	Name    string
	Options work.JobOptions
	Handler JobHandler
}

type JobRunner struct {
	jobs        []Job
	middlewares []JobMiddleware
}

func NewJobRunner() *JobRunner {
	jobs := make([]Job, 0)
	middlewares := make([]JobMiddleware, 0)
	return &JobRunner{
		jobs:        jobs,
		middlewares: middlewares,
	}
}

func (r *JobRunner) UseJob(jobs ...Job) {
	r.jobs = append(r.jobs, jobs...)
}

func (r *JobRunner) UseJobMiddleware(middlewares ...JobMiddleware) {
	r.middlewares = append(r.middlewares, middlewares...)
}

func (r *JobRunner) init(pool *work.WorkerPool) {
	for _, job := range r.jobs {
		jobHandler := job.Handler
		middlewares := r.middlewares
		for i := len(middlewares) - 1; i >= 0; i-- {
			jobHandler = middlewares[i](jobHandler)
		}
		pool.JobWithOptions(job.Name, job.Options, jobHandler)
	}
}

func (r *JobRunner) Start(concurrency uint) {
	redisPool := InitRedisPool()
	pool := work.NewWorkerPool(Context{}, concurrency, viper.GetString("queue.redis.prefix"), redisPool)

	r.init(pool)

	pool.Start()
}
