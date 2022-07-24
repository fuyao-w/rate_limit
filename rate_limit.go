package rate_limit

/*
	Copyright [2022] [wangfuyao]

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/
import (
	"time"
)

type RateLimit interface {
	Available() (available int64)
	Take(count int64) error
	TakeAvailable(count int64) (realCount int64)
	TryTake(count int64, maxWait time.Duration) (succ bool)
}
