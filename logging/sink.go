package logging

import "github.com/retail-ai-inc/bean/v2/logging/types"

type Sink interface {
	Write(entry types.Entry) error
}
