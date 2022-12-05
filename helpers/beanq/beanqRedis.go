package beanq

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
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
	wg     *sync.WaitGroup
	ch     chan redis.XStream

	broker                   string
	keepJobInQueue           time.Duration
	keepFailedJobsInHistory  time.Duration
	keepSuccessJobsInHistory time.Duration

	minWorkers  int
	jobMaxRetry int
	prefix      string
}

func NewRedis(options Options) *BeanqRedis {
	ctx := context.Background()
	once.Do(func() {
		client = redis.NewClient(options.RedisOptions)
	})
	return &BeanqRedis{
		client:                   client,
		ctx:                      ctx,
		wg:                       &sync.WaitGroup{},
		ch:                       make(chan redis.XStream),
		minWorkers:               options.MinWorkers,
		jobMaxRetry:              options.JobMaxRetry,
		prefix:                   options.Prefix,
		keepJobInQueue:           options.KeepJobInQueue,
		keepFailedJobsInHistory:  options.KeepFailedJobsInHistory,
		keepSuccessJobsInHistory: options.KeepSuccessJobsInHistory,
	}
}
func (t *BeanqRedis) DelayPublish(task *Task, option ...Option) (*Result, error) {
	return t.Publish(task, option...)
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
	values := make(map[string]any)
	values["payload"] = task.payload
	values["addtime"] = time.Now()

	if opt.executeTime != 0 {
		values["executeTime"] = opt.executeTime
	}

	strcmd := t.client.XAdd(t.ctx, &redis.XAddArgs{
		Stream:     opt.queue,
		NoMkStream: false,
		MaxLen:     opt.maxLen,
		MinID:      "",
		Approx:     false,
		//Limit:      0,
		ID:     id,
		Values: values,
	})
	if err := strcmd.Err(); err != nil {
		return nil, err
	}

	return &Result{Args: strcmd.Args(), Id: strcmd.Val()}, nil
}
func (t *BeanqRedis) Run(server *Server) {
	consumers := server.consumers()

	workers := make(chan struct{}, t.minWorkers)

	for _, v := range consumers {

		err := t.createGroup(v.queue, v.group)
		if err != nil {
			fmt.Printf("CreateGroupErr:%+v \n", err)
			continue
		}

		workers <- struct{}{}
		go func(handler *consumerHandler) {
			err := t.readGroups(handler.queue, handler.group, server.count)
			if err != nil {
				fmt.Printf("ReadGroup Error:%s \n", err.Error())
				return
			}
			t.consumerMsgs(handler.consumerFun, handler.group)
			<-workers
		}(v)
	}
	//   https://redis.io/commands/xclaim/
	//
	go t.claim(consumers)

	select {}
}

/*
  - claim
  - @Description:
    need test
    this function can't work,developing
  - @receiver t
*/
func (t *BeanqRedis) claim(consumers []*consumerHandler) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			start := "-"
			end := "+"

			for _, consumer := range consumers {

				res, err := t.client.XPendingExt(t.ctx, &redis.XPendingExtArgs{
					Stream: consumer.queue,
					Group:  consumer.group,
					Start:  start,
					End:    end,
					Count:  10,
				}).Result()
				if err != nil && err != redis.Nil {
					fmt.Printf("PendError:%s \n", err.Error())
					break
				}
				for _, v := range res {
					if v.Idle.Seconds() >= 10 {
						claims, err := t.client.XClaim(t.ctx, &redis.XClaimArgs{
							Stream:   consumer.queue,
							Group:    consumer.group,
							Consumer: consumer.queue,
							MinIdle:  10 * time.Second,
							Messages: []string{v.ID},
						}).Result()
						if err != nil && err != redis.Nil {
							fmt.Printf("ClaimError:%s \n", err.Error())
							continue
						}
						if err := t.client.XAck(t.ctx, consumer.queue, consumer.group, v.ID).Err(); err != nil {
							continue
						}
						fmt.Printf("claim:%+v \n", claims)
						t.ch <- redis.XStream{
							Stream:   consumer.queue,
							Messages: claims,
						}
					}
				}
			}
		}
	}
}
func (t *BeanqRedis) readGroups(queue, group string, count int64) error {
	//consumerUuid, err := uuid.NewUUID()
	//if err != nil {
	//	return nil, err
	//}

	go func() {

		for {
			select {
			default:
				streams, err := t.client.XReadGroup(t.ctx, &redis.XReadGroupArgs{
					Group:    group,
					Streams:  []string{queue, ">"},
					Consumer: queue,
					Count:    count,
					Block:    0,
				}).Result()
				if err != nil {
					fmt.Printf("readgroup:%+v \n", err.Error())
					continue
				}
				if len(streams) <= 0 {
					continue
				}

				for _, v := range streams {
					t.ch <- v
				}
			}
		}
	}()
	return nil
}
func (t *BeanqRedis) consumerMsgs(f DoConsumer, group string) {
	flag := SuccessInfo
	result := &ConsumerResult{
		Level:   InfoLevel,
		Info:    flag,
		RunTime: "",
	}
	var now time.Time

	for {
		select {
		case msg := <-t.ch:

			task := &Task{
				name: msg.Stream,
			}
			for _, vm := range msg.Messages {

				task.id = vm.ID
				if payload, ok := vm.Values["payload"]; ok {
					task.payload = stringx.StringToByte(payload.(string))
				}
				if addtime, ok := vm.Values["addtime"]; ok {
					task.addTime = cast.ToTime(addtime)
				}
				fmt.Printf("task1:%+v \n", msg)
				now = time.Now()
				if executeT, ok := vm.Values["executeTime"]; ok {
					if cast.ToInt64(executeT) > now.Unix() {
						continue
					}
				}
				fmt.Printf("task2:%+v \n", msg)
				err := f(task, t.client)
				if err != nil {
					flag = FailedInfo
					result.Level = ErrLevel
					result.Info = flagInfo(err.Error())
				}

				sub := time.Now().Sub(now)

				result.Payload = stringx.ByteToString(task.payload)
				result.AddTime = time.Now().Format(timex.DateTime)
				result.RunTime = sub.String()
				result.Queue = msg.Stream
				result.Group = group

				b, err := json.Marshal(result)
				if err != nil {
					fmt.Printf("JsonMarshal Error:%s \n", err.Error())
					continue
				}

				//ack
				if err := t.client.XAck(t.ctx, msg.Stream, group, vm.ID).Err(); err != nil {
					fmt.Printf("ACK Error:%s \n", err.Error())
					continue
				}

				if err := t.client.LPush(t.ctx, string(flag), b).Err(); err != nil {
					fmt.Printf("LPUSH ERROR:%+v \n", err)
					continue
				}
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

	err := t.client.XGroupCreateMkStream(t.ctx, queue, group, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
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
