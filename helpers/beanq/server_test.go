package beanq

import (
	"fmt"
	"github.com/go-redis/redis/v8"
	"testing"
)

func TestConsumer(t *testing.T) {
	rdb := NewRdb(options)
	for i := 0; i < 10; i++ {
		go func() {
			err := rdb.Consumer(group, queue, func(stream []redis.XStream, r *redis.Client) error {
				fmt.Printf("%+v \n", stream)
				return nil
			})
			if err != nil {
				t.Log(err.Error())
			}
		}()
	}
	select {}
}
