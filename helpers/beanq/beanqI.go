package beanq

import (
	"github.com/go-redis/redis/v8"
)

type Beanq interface {
	Publish(task *Task, option ...Option) (*Result, error)
	Run(server *Server)
	Close() error
}

//
//  ConsumerResult
//  @Description:

type ConsumerResult struct {
	Level       string
	Info        string
	AddTime     string
	ExecuteTime string
}

// need more parameters
type Result struct {
	Id   string
	Args []any
}
type Options struct {
	RedisOptions *redis.Options
}

// only use to test
func NewBeanq(broker string, options Options) Beanq {
	if broker == "redis" {
		return NewRedis(options.RedisOptions)
	}
	return nil
}
