package beanq

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/retail-ai-inc/bean/helpers/beanq/json"
	"github.com/spf13/cast"
	"log"
	"testing"
	"time"
)

var (
	options = Options{
		RedisOptions: &redis.Options{
			Addr:      "localhost:6381",
			Dialer:    nil,
			OnConnect: nil,
			Username:  "",
			Password:  "secret",
			DB:        2,
		},
	}
	queue    = "ch2"
	group    = "g2"
	consumer = "cs1"
	clt      Beanq
)

func init() {
	clt = NewBeanq("redis", options)
}

/*
  - TestPublish
  - @Description:
    publisher
  - @param t
*/
func TestPublish1(t *testing.T) {

	for i := 0; i < 5; i++ {
		m := make(map[int]string)
		m[i] = "k" + cast.ToString(i)

		d, _ := json.Marshal(m)
		task := NewTask("", d)
		cmd, err := clt.Publish(task, Queue("ch2"), Group("g2"))
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Printf("%+v \n", cmd)
	}

	defer clt.Close()

}

func TestXInfo(t *testing.T) {
	ctx := context.Background()

	clt := NewRedis(options)
	cmd := clt.client.XInfoStream(ctx, queue)
	fmt.Printf("%+v \n", cmd.Val())
	groupCmd := clt.client.XInfoGroups(ctx, queue)
	fmt.Printf("%+v \n", groupCmd.Val())
}
func TestPending(t *testing.T) {
	ctx := context.Background()
	clt := NewRedis(options)

	cmd := clt.client.XPending(ctx, queue, group)
	fmt.Printf("%+v \n", cmd.Val())
}
func TestInfo(t *testing.T) {
	ctx := context.Background()
	clt := NewRedis(options)

	cmd := clt.client.Info(ctx)

	fmt.Printf("%+v \n", cmd.Val())
}
func TestMemoryUsage(t *testing.T) {
	ctx := context.Background()
	clt := NewRedis(options)
	cmd := clt.client.MemoryUsage(ctx, "success")
	fmt.Printf("%+v \n", cmd)
}
func TestRetry(t *testing.T) {
}

var retryFlag chan bool = make(chan bool)

func retry(f func() bool, delayTime int) {
	index := 0
	for {
		go time.AfterFunc(time.Duration(delayTime)*time.Second, func() {
			retryFlag <- f()
		})
		if <-retryFlag {
			return
		}
		if index == 3 {
			return
		}
		index++
	}
}
