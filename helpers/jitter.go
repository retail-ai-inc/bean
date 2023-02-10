package helpers

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

var rnd = newRnd()
var rndMu sync.Mutex

// JitterBackoff Return capped exponential backoff with jitter. It is useful for http client when you want to retry request.
// http://www.awsarchitectureblog.com/2015/03/backoff.html

// An example use case:

// for i := 0; i <= retryCount; i++ {
//     resp, err := http.Get("https://retail-ai.jp")
//     if err == nil {
//       return nil
//     }

//     // Don't need to wait when no retries left.
//     if i == retryCount {
//       return err
//     }

//     waitTime := helpers.JitterBackoff(time.Duration(100) * time.Millisecond, time.Duration(2000) * time.Millisecond, i)

//     select {
//     case <-time.After(waitTime):
//     case <-c.Done():
//       return c.Err()
//     }
// }
func JitterBackoff(min, max time.Duration, attempt int) time.Duration {
	base := float64(min)
	capLevel := float64(max)

	temp := math.Min(capLevel, base*math.Exp2(float64(attempt)))
	ri := time.Duration(temp / 2)
	result := randDuration(ri)

	if result < min {
		result = min
	}

	return result
}

func randDuration(center time.Duration) time.Duration {
	rndMu.Lock()
	defer rndMu.Unlock()

	var ri = int64(center)
	var jitter = rnd.Int63n(ri)
	return time.Duration(math.Abs(float64(ri + jitter)))
}

func newRnd() *rand.Rand {
	var seed = time.Now().UnixNano()
	var src = rand.NewSource(seed)
	return rand.New(src)
}
