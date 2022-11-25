package beanq

import (
	"fmt"
	"github.com/go-redis/redis/v8"
	"testing"
)

func TestConsumer(t *testing.T) {
	rdb := NewBeanq(2, options)
	server := NewServer()
	server.Register(group, queue, func(task *Task, r *redis.Client) error {
		fmt.Printf("载荷：%+v \n", task.Payload())
		return nil
	})
	//add new job
	//server.Register("group2","queue2", func(task *Task, r *redis.Client) error {
	//	return nil
	//})
	rdb.Run(server)

}
