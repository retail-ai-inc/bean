package beanq

import "sync"

type consumerHandler struct {
	group, queue string
	consumerFun  DoConsumer
}
type Server struct {
	mu    sync.RWMutex
	m     []*consumerHandler
	count int64
}

func NewServer(count int64) *Server {
	if count == 0 {
		count = 10
	}
	return &Server{count: count}
}
func (t *Server) Register(group, queue string, consumerFun DoConsumer) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if group == "" {
		group = defaultOptions.defaultGroup
	}
	if queue == "" {
		queue = defaultOptions.defaultQueueName
	}
	t.m = append(t.m, &consumerHandler{
		group:       group,
		queue:       queue,
		consumerFun: consumerFun,
	})
}
func (t *Server) consumers() []*consumerHandler {
	return t.m
}
