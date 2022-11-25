package beanq

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/retail-ai-inc/bean/helpers/beanq/json"
	"log"
	"testing"
	"time"
)

var (
	options = &redis.Options{
		Addr:      "localhost:6381",
		Dialer:    nil,
		OnConnect: nil,
		Username:  "",
		Password:  "secret",
		DB:        1,
	}

	queue    = "ch"
	group    = "g"
	consumer = "cs1"
)

func TestPublish(t *testing.T) {

	var data = struct {
		Key string
		Val string
	}{
		"k1",
		"v1",
	}
	d, _ := json.Json.Marshal(data)
	task := NewTask("", d)
	clt := NewRdb(2, options)

	for i := 0; i < 10; i++ {
		cmd, err := clt.Publish(task, Queue("ch"), Group("g"))
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Printf("%+v \n", cmd)
	}

	defer clt.Close()

}
func TestXInfo(t *testing.T) {
	ctx := context.Background()
	clt := NewRdb(2, options)
	cmd := clt.client.XInfoStream(ctx, queue)
	fmt.Printf("%+v \n", cmd.Val())
	groupCmd := clt.client.XInfoGroups(ctx, queue)
	fmt.Printf("%+v \n", groupCmd.Val())
}
func TestPending(t *testing.T) {
	ctx := context.Background()
	clt := NewRdb(2, options)

	cmd := clt.client.XPending(ctx, queue, group)
	fmt.Printf("%+v \n", cmd.Val())
}
func TestInfo(t *testing.T) {
	ctx := context.Background()
	clt := NewRdb(2, options)

	cmd := clt.client.Info(ctx)

	fmt.Printf("%+v \n", cmd.Val())
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
