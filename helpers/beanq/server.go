package beanq

import "sync"

type consumerHandler struct {
	group, queue string
	consumer     DoConsumer
}
type Server struct {
	mu    sync.RWMutex
	m     []*consumerHandler
	count int64
}

func NewServer(count int64) *Server {
	return &Server{count: count}
}
func (t *Server) Register(group, queue string, consumer DoConsumer) {

	if group == "" {
		group = defaultGroup
	}
	if queue == "" {
		queue = defaultQueueName
	}
	t.m = append(t.m, &consumerHandler{
		group:    group,
		queue:    queue,
		consumer: consumer,
	})
}
func (t *Server) consumers() []*consumerHandler {
	return t.m
}
