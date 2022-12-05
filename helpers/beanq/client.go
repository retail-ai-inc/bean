package beanq

type OptionType int

const (
	MaxRetryOpt OptionType = iota
	QueueOpt
	GroupOpt
	MaxLenOpt
	ExecuteTimeOpt
)

type option struct {
	retry       int
	queue       string
	group       string
	maxLen      int64
	executeTime int64
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
	executeTime  int64
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
	return "queueOption"
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
	return "retryOption"
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
	return "groupOption"
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
	return "maxLenOption"
}
func (ml maxLenOption) OptType() OptionType {
	return MaxLenOpt
}
func (ml maxLenOption) Value() any {
	return int(ml)
}

/*
* ExecuteTime
*  @Description:
* @param tm
* @return Option
 */
func ExecuteTime(unixTime int64) Option {
	return executeTime(unixTime)
}
func (et executeTime) String() string {
	return "executeTime"
}
func (et executeTime) OptType() OptionType {
	return ExecuteTimeOpt
}
func (et executeTime) Value() any {
	return int64(et)
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
		switch f.OptType() {
		case QueueOpt:
			res.queue = f.Value().(string)
		case GroupOpt:
			res.group = f.Value().(string)
		case MaxRetryOpt:
			res.retry = f.Value().(int)
		case MaxLenOpt:
			res.maxLen = f.Value().(int64)
		case ExecuteTimeOpt:
			res.executeTime = f.Value().(int64)
		}
	}
	return res, nil
}
