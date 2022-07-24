package rate_limit

import "time"

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

// Clock 时钟 用户可以传入自定义的时钟用于mock测试
type Clock interface {
	Now() time.Time
	Sleep(duration time.Duration)
}

// standardClock 普通的时钟实现
type standardClock struct{}

func (s *standardClock) Now() time.Time {
	return time.Now()
}

func (s *standardClock) Sleep(duration time.Duration) {
	time.Sleep(duration)
}
