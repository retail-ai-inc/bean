package logging

import "github.com/retail-ai-inc/bean/v2/logging/types"

type Processor interface {
	Process(entry types.Entry) types.Entry
}
