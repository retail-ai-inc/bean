package beanq

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/retail-ai-inc/bean/helpers/beanq/json"
	"sync"
	"time"
)

var (
	once   sync.Once
	client *redis.Client
)

type DoConsumer func([]redis.XStream, *redis.Client) error

type Rdb struct {
	client *redis.Client
	ctx    context.Context
}

func NewRdb(options *redis.Options) *Rdb {
	once.Do(func() {
		client = redis.NewClient(options)
	})

	return &Rdb{client: client, ctx: context.Background()}
}
func (t *Rdb) Publish(queue, group string, data any) (*redis.StringCmd, error) {

	d, err := json.Json.MarshalToString(data)
	if err != nil {
		return nil, err
	}

	strcmd := t.client.XAdd(t.ctx, &redis.XAddArgs{
		Stream: queue,
		Limit:  0,
		MaxLen: 0,
		Values: map[string]any{"arg": d},
	})
	if strcmd.Err() != nil {
		return nil, strcmd.Err()
	}
	if err := t.createGroup(queue, group); err != nil {
		return nil, err
	}
	return strcmd, nil

}

func (t *Rdb) Consumer(group, queue string, f DoConsumer) error {

	consumerUuid, err := uuid.NewUUID()
	if err != nil {
		return err
	}

	ch := make(chan []redis.XStream)

	go func() {
		for {
			cmd := t.client.XReadGroup(t.ctx, &redis.XReadGroupArgs{
				Group:    group,
				Streams:  []string{queue, ">"},
				Consumer: consumerUuid.String(),
				Count:    2,
				Block:    0,
			})
			if cmd.Err() != nil {
				err = cmd.Err()
				fmt.Printf("CMD Error:%+v \n", err)
				continue
			}
			if len(cmd.Val()) <= 0 {
				continue
			}
			ch <- cmd.Val()
		}
	}()
	if err != nil {
		return err
	}
	for v := range ch {
		now := time.Now()
		err := f(v, t.client)
		if err != nil {
			fmt.Printf("Error:%+v \n", err)
			continue
		}
		sub := time.Now().Sub(now)
		fmt.Printf("ExecutionTime:%+v,Consumer:%s \n", sub, consumerUuid)
	}

	return nil
}

/*
  - createGroup
  - @Description:
    if group not exist,then create it
  - @receiver t
  - @param queue
  - @param group
  - @return error
*/
func (t *Rdb) createGroup(queue, group string) error {
	groupCmd := t.client.XInfoGroups(t.ctx, queue)
	if groupCmd.Err() != nil {
		return groupCmd.Err()
	}
	var b bool
	vals := groupCmd.Val()
	for _, val := range vals {
		if val.Name == group {
			b = true
			break
		}
	}
	if b {
		return nil
	}

	if err := t.client.XGroupCreate(t.ctx, queue, group, "0").Err(); err != nil {
		return err
	}

	return nil
}

/*
  - Close
  - @Description:
    close redis client
  - @receiver t
  - @return error
*/
func (t *Rdb) Close() error {
	return t.client.Close()
}
