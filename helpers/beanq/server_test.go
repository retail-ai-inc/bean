package beanq

import (
	"fmt"
	"github.com/go-redis/redis/v8"
	"testing"
)

//发布新消息，需要确定消费者组，是否有多个客户端的情况。

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

		fmt.Printf("1PayLoad：%+v \n", task.Payload())
		return nil
	})
	rdb.Run(server)

}
func TestConsumer2(t *testing.T) {

	rdb := NewBeanq("redis", options)

	server := NewServer(3)
	server.Register(group, queue, func(task *Task, r *redis.Client) error {
		fmt.Printf("2PayLoad:%+v \n", task.Payload())
		return nil
	})
	rdb.Run(server)
}
