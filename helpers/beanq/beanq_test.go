package beanq

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"log"
	"testing"
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
	group    = "ch-group"
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

	clt := NewRdb(options)
	for i := 0; i < 10; i++ {
		cmd, err := clt.Publish(queue, group, data)
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Printf("%+v \n", cmd)
	}

	defer clt.Close()

}
func TestXInfo(t *testing.T) {
	ctx := context.Background()
	clt := NewRdb(options)
	cmd := clt.client.XInfoStream(ctx, queue)
	fmt.Printf("%+v \n", cmd.Val())
	groupCmd := clt.client.XInfoGroups(ctx, queue)
	fmt.Printf("%+v \n", groupCmd.Val())
}
func TestPending(t *testing.T) {
	ctx := context.Background()
	clt := NewRdb(options)

	cmd := clt.client.XPending(ctx, queue, group)
	fmt.Printf("%+v \n", cmd.Val())
}
func TestInfo(t *testing.T) {
	ctx := context.Background()
	clt := NewRdb(options)
	cmd := clt.client.Info(ctx)
	fmt.Printf("%+v \n", cmd.Val())
}
