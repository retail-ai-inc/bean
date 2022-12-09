package beanq

import (
	"github.com/go-redis/redis/v8"
	"log"
	"time"
)

type Beanq interface {
	Publish(task *Task, option ...Option) (*Result, error)
	DelayPublish(task *Task, delayTime time.Time, option ...Option) (*Result, error)
	Run(server *Server)
	Close() error
}

type flagInfo string
type levelMsg string

const (
	SuccessInfo flagInfo = "success"
	FailedInfo  flagInfo = "failed"

	ErrLevel  levelMsg = "error"
	InfoLevel levelMsg = "info"
)

//
//  ConsumerResult
//  @Description:

type ConsumerResult struct {
	Level   levelMsg
	Info    flagInfo
	Payload any

	AddTime string
	RunTime string

	Queue, Group, Consumer string
}

// need more parameters
type Result struct {
	Id   string
	Args []any
}
type Options struct {
	RedisOptions *redis.Options

	KeepJobInQueue           time.Duration
	KeepFailedJobsInHistory  time.Duration
	KeepSuccessJobsInHistory time.Duration

	MinWorkers  int
	JobMaxRetry int
	Prefix      string

	defaultQueueName, defaultGroup string
	defaultMaxLen                  int64

	defaultDelayQueueName, defaultDelayGroup string
}

var defaultOptions = &Options{
	KeepJobInQueue:           7 * 1440 * time.Minute,
	KeepFailedJobsInHistory:  7 * 1440 * time.Minute,
	KeepSuccessJobsInHistory: 7 * 1440 * time.Minute,
	MinWorkers:               10,
	JobMaxRetry:              3,
	Prefix:                   "beanq",

	defaultQueueName: "default-queue",
	defaultGroup:     "default-group",
	defaultMaxLen:    1000,

	defaultDelayQueueName: "default-delay-queue",
	defaultDelayGroup:     "default-delay-group",
}

// only use to test
func NewBeanq(broker string, options Options) Beanq {
	if options.KeepJobInQueue == 0 {
		options.KeepJobInQueue = defaultOptions.KeepJobInQueue
	}
	if options.KeepFailedJobsInHistory == 0 {
		options.KeepFailedJobsInHistory = defaultOptions.KeepFailedJobsInHistory
	}
	if options.KeepSuccessJobsInHistory == 0 {
		options.KeepSuccessJobsInHistory = defaultOptions.KeepSuccessJobsInHistory
	}
	if options.MinWorkers == 0 {
		options.MinWorkers = defaultOptions.MinWorkers
	}
	if options.JobMaxRetry == 0 {
		options.JobMaxRetry = defaultOptions.JobMaxRetry
	}
	if options.Prefix == "" {
		options.Prefix = defaultOptions.Prefix
	}
	if broker == "redis" {
		if options.RedisOptions == nil {
			log.Fatalln("Missing Redis configuration")
		}
		return NewRedis(options)
	}
	return nil
}
