// MIT License

// Copyright (c) The RAI Authors

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package helpers

import (
	"math"
	"math/rand"
	"time"
)

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

//	    select {
//	    case <-time.After(waitTime):
//	    case <-c.Done():
//	      return c.Err()
//	    }
//	}
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
	var ri = int64(center)
	var jitter = rand.Int63n(ri)
	return time.Duration(math.Abs(float64(ri + jitter)))
}
