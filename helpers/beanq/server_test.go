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
	rdb := NewBeanq("redis", Options{RedisOptions: options})
	server := NewServer(3)
	server.Register(group, queue, func(task *Task, r *redis.Client) error {
		fmt.Printf("PayLoad：%+v \n", task.Payload())
		fmt.Printf("AddTime :%+v \n", task.AddTime())
		return nil
	})
	//add new job
	//server.Register("group2","queue2", func(task *Task, r *redis.Client) error {
	//	return nil
	//})
	//add other job
	//server.Register("group3","queue3", func(task *Task, r *redis.Client) error {
	//	return nil
	//})
	rdb.Run(server)

}
