package beanq

import (
	"fmt"
	"github.com/go-redis/redis/v8"
	"testing"
)

/*
  - TestConsumer
  - @Description:
    consumer
  - @param t
*/
func TestConsumer(t *testing.T) {
	rdb := NewBeanq("redis", options)

	server := NewServer(3)
	server.Register(group, queue, func(task *Task, r *redis.Client) error {

		fmt.Printf("PayLoad：%+v \n", task.Payload())
		return nil
	})
	server.Register(defaultOptions.defaultDelayGroup, defaultOptions.defaultDelayQueueName, func(task *Task, r *redis.Client) error {
		fmt.Printf("Delay:%+v \n", task.Payload())
		return nil
	})
	rdb.Run(server)

}
func TestConsumer2(t *testing.T) {

	rdb := NewBeanq("redis", options)

	server := NewServer(3)
	server.Register("g11", "c11", func(task *Task, r *redis.Client) error {
		fmt.Printf("2PayLoad:%+v \n", task.Payload())
		return nil
	})
	rdb.Run(server)
}
func TestDelayConsumer(t *testing.T) {
	rdb := NewRedis(options)
	rdb.delayConsumer()
}
