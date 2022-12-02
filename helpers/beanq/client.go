package beanq

type OptionType int

const (
	MaxRetryOpt OptionType = iota
	QueueOpt
	GroupOpt
	MaxLenOpt
)

type option struct {
	retry  int
	queue  string
	group  string
	maxLen int64
}

type Option interface {
	String() string
	OptType() OptionType
	Value() any
}

type (
	retryOption  int
	queueOption  string
	groupOption  string
	maxLenOption int64
)

/*
* Queue
*  @Description:
* @param name
* @return Option
 */
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

/*
* Retry
*  @Description:
* @param retries
* @return Option
 */
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

/*
* Group
*  @Description:
* @param name
* @return Option
 */
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

/*
* MaxLen
*  @Description:
* @param maxlen
* @return Option
 */
func MaxLen(maxlen int) Option {
	return maxLenOption(maxlen)
}
func (ml maxLenOption) String() string {
	return ""
}
func (ml maxLenOption) OptType() OptionType {
	return MaxLenOpt
}
func (ml maxLenOption) Value() any {
	return int(ml)
}

/*
* composeOptions
*  @Description:
* @param options
* @return option
* @return error
 */
func composeOptions(options ...Option) (option, error) {
	res := option{
		retry:  defaultOptions.JobMaxRetry,
		queue:  defaultOptions.defaultQueueName,
		group:  defaultOptions.defaultGroup,
		maxLen: defaultOptions.defaultMaxLen,
	}
	for _, f := range options {
		switch opt := f.(type) {
		case queueOption:
			res.queue = string(opt)
		case retryOption:
			res.retry = int(opt)
		case groupOption:
			res.group = string(opt)
		case maxLenOption:
			res.maxLen = int64(opt)
		default:

		}
	}
	return res, nil
}
