package beanq

import (
	"fmt"
	"github.com/go-redis/redis/v8"
	"testing"
)

func TestConsumer(t *testing.T) {
	rdb := NewRdb(2, options)
	//for i := 0; i < 10; i++ {
	//	go func() {
	err := rdb.Consumer(group, queue, func(task *Task, r *redis.Client) error {
		fmt.Printf("%+v \n", task.Payload())
		return nil
	})
	if err != nil {
		t.Log(err.Error())
	}
	//	}()
	//}
	//select {}
}
