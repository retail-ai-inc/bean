package beanq

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/retail-ai-inc/bean/helpers/beanq/json"
	"log"
	"sync"
)

var (
	once   sync.Once
	client *redis.Client
)

type Data struct {
	Key string
	Val string
}
type Rdb struct {
	client *redis.Client
	ctx    context.Context
}

func NewRdb(addr, password string, db int) *Rdb {
	once.Do(func() {
		options := &redis.Options{
			Addr:      addr,
			Dialer:    nil,
			OnConnect: nil,
			Username:  "",
			Password:  password,
			DB:        db,
		}
		client = redis.NewClient(options)
	})

	return &Rdb{client: client, ctx: context.Background()}
}
func (t *Rdb) Publish(queue string, data Data) (*redis.StringCmd, error) {

	d, err := json.Json.MarshalToString(data)
	if err != nil {
		return nil, err
	}

	strcmd := t.client.XAdd(t.ctx, &redis.XAddArgs{
		Stream: queue,
		Limit:  0,
		Values: map[string]string{"arg": d},
	})
	if strcmd.Err() != nil {
		//return nil, fmt.Errorf("Error:%s,Stack:%s", "add error", debug.Stack())
		return nil, strcmd.Err()
	}
	return strcmd, nil
}
func (t *Rdb) Consumer(group, queue, consumer string) chan []redis.XStream {

	ch := make(chan []redis.XStream)
	go func() {
		for {
			res, err := t.client.XReadGroup(t.ctx, &redis.XReadGroupArgs{
				Group:    group,
				Streams:  []string{queue, ">"},
				Consumer: consumer,
				Count:    10,
			}).Result()
			if err != nil {
				log.Fatalln(err.Error())
			}
			ch <- res
		}
	}()
	return ch
}
