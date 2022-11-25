package beanq

const (
	defaultQueueName = "default"
	defaultRetry     = 0
	defaultGroup     = "default-group"
)

type OptionType int

const (
	MaxRetryOpt OptionType = iota
	QueueOpt
	GroupOpt
)

type option struct {
	retry int
	queue string
	group string
}

type Option interface {
	String() string
	OptType() OptionType
	Value() any
}

type (
	retryOption int
	queueOption string
	groupOption string
)

func Queue(name string) Option {
	return queueOption(name)
}
func (queue queueOption) String() string {
	return ""
}
func (queue queueOption) OptType() OptionType {
	return QueueOpt
}
func (queue queueOption) Value() any {
	return string(queue)
}

func Retry(retries int) Option {
	return retryOption(retries)
}
func (retry retryOption) String() string {
	return ""
}
func (retry retryOption) OptType() OptionType {
	return MaxRetryOpt
}
func (retry retryOption) Value() any {
	return int(retry)
}

func Group(name string) Option {
	return groupOption(name)
}
func (group groupOption) String() string {
	return ""
}
func (group groupOption) OptType() OptionType {
	return GroupOpt
}
func (group groupOption) Value() any {
	return string(group)
}

func composeOptions(options ...Option) (option, error) {
	res := option{
		retry: defaultRetry,
		queue: defaultQueueName,
		group: defaultGroup,
	}
	for _, f := range options {
		switch opt := f.(type) {
		case queueOption:
			res.queue = string(opt)
		case retryOption:
			res.retry = int(opt)
		case groupOption:
			res.group = string(opt)
		default:

		}
	}
	return res, nil
}
