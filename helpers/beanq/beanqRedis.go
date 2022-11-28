package beanq

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/retail-ai-inc/bean/helpers/beanq/json"
	"github.com/retail-ai-inc/bean/helpers/beanq/stringx"
	"github.com/retail-ai-inc/bean/helpers/beanq/timex"
	"github.com/spf13/cast"
	"sync"
	"time"
)

var (
	once   sync.Once
	client *redis.Client
)

type DoConsumer func(*Task, *redis.Client) error

type BeanqRedis struct {
	client *redis.Client
	ctx    context.Context
	wg     sync.WaitGroup

	broker                   string
	keepJobInQueue           time.Duration
	keepFailedJobsInHistory  time.Duration
	keepSuccessJobsInHistory time.Duration

	minWorkers  int
	jobMaxRetry int
	prefix      string
}

func NewRedis(options *redis.Options) *BeanqRedis {
	ctx := context.Background()
	once.Do(func() {
		client = redis.NewClient(options)
	})
	return &BeanqRedis{client: client, ctx: ctx, minWorkers: 10}
}
func (t *BeanqRedis) DelayPublish(task *Task, option ...Option) (*Result, error) {
	return nil, nil
}
func (t *BeanqRedis) Publish(task *Task, option ...Option) (*Result, error) {

	opt, err := composeOptions(option...)
	if err != nil {
		return nil, err
	}
	id := task.name
	if id == "" {
		id = "*"
	}
	strcmd := t.client.XAdd(t.ctx, &redis.XAddArgs{
		Stream:     opt.queue,
		NoMkStream: false,
		MaxLen:     1000,
		MinID:      "",
		Approx:     true,
		//Limit:      0,
		ID:     id,
		Values: map[string]any{"payload": task.payload, "addtime": time.Now()},
	})
	if err := strcmd.Err(); err != nil {
		return nil, err
	}

	if err := t.createGroup(opt.queue, opt.group); err != nil {
		return nil, err
	}
	return &Result{Args: strcmd.Args(), Id: strcmd.Val()}, nil
}
func (t *BeanqRedis) Run(server *Server) {
	consumers := server.consumers()

	ch := make(chan struct{}, t.minWorkers)

	//need to control the number of goroutine
	//will optimize
	for _, v := range consumers {
		ch <- struct{}{}
		t.wg.Add(1)

		go func(handler *consumerHandler) {
			defer t.wg.Done()
			chs, err := t.readGroups(handler.queue, handler.group, server.count)
			if err != nil {
				fmt.Printf("ReadGroup Error:%s \n", err.Error())
			}
			t.consumerMsgs(chs, handler.consumer, handler.group)
			<-ch
		}(v)

		t.wg.Wait()
	}
	select {}
}
func (t *BeanqRedis) readGroups(queue, group string, count int64) (<-chan redis.XStream, error) {
	consumerUuid, err := uuid.NewUUID()
	if err != nil {
		return nil, err
	}

	ch := make(chan redis.XStream)
	go func() {
		for {
			cmd := t.client.XReadGroup(t.ctx, &redis.XReadGroupArgs{
				Group:    group,
				Streams:  []string{queue, ">"},
				Consumer: consumerUuid.String(),
				Count:    count,
				Block:    0,
			})
			err = cmd.Err()
			if err != nil {
				continue
			}
			if len(cmd.Val()) <= 0 {
				continue
			}
			for _, v := range cmd.Val() {
				ch <- v
			}
		}
	}()
	return ch, nil
}
func (t *BeanqRedis) consumerMsgs(ch <-chan redis.XStream, f DoConsumer, group string) {
	result := &ConsumerResult{
		Level:       "info",
		Info:        "success",
		ExecuteTime: "",
	}
	flag := "success"
	var now time.Time

	for v := range ch {
		task := &Task{
			name: v.Stream,
		}
		for _, vm := range v.Messages {

			task.id = vm.ID
			if payload, ok := vm.Values["payload"]; ok {
				task.payload = stringx.StringToByte(payload.(string))
			}
			if addtime, ok := vm.Values["addtime"]; ok {
				task.addTime = cast.ToTime(addtime)
			}
			now = time.Now()

			err := f(task, t.client)
			if err != nil {
				flag = "failed"
				result.Level = "error"
				result.Info = err.Error()
			}

			sub := time.Now().Sub(now)

			result.AddTime = time.Now().Format(timex.DateTime)
			result.ExecuteTime = sub.String()

			b, err := json.Json.Marshal(result)
			if err != nil {
				fmt.Printf("JsonMarshal Error:%s \n", err.Error())
				continue
			}
			//ack
			if err := t.client.XAck(t.ctx, v.Stream, group, vm.ID).Err(); err != nil {
				fmt.Printf("ACK Error:%s \n", err.Error())
				continue
			}

			if err := t.client.LPush(t.ctx, flag, b).Err(); err != nil {
				fmt.Printf("LPUSH ERROR:%+v \n", err)
				continue
			}
		}
	}
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
func (t *BeanqRedis) createGroup(queue, group string) error {
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
func (t *BeanqRedis) Close() error {
	return t.client.Close()
}
