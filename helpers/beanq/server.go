package beanq

import "sync"

type consumerHandler struct {
	group, queue string
	consumer     DoConsumer
}
type Server struct {
	mu sync.RWMutex
	m  []*consumerHandler
}

func NewServer() *Server {
	return &Server{}
}
func (t *Server) Register(group, queue string, consumer DoConsumer) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	t.m = append(t.m, &consumerHandler{
		group:    group,
		queue:    queue,
		consumer: consumer,
	})
}
func (t *Server) consumers() []*consumerHandler {
	return t.m
}
