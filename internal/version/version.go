/*
 Copyright 2024 Qiniu Cloud (qiniu.com).

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

package version

import (
	"fmt"
	"runtime/debug"
)

var (
	defaultVersion = "UNSTABLE"
	version        = ""
)

func Version() string {
	if version != "" && version != defaultVersion {
		return version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Println(ok)
		return defaultVersion
	}

	if info.Main.Version == "(devel)" {
		fmt.Println(info.Main.Version)
		return defaultVersion
	}

	return info.Main.Version
}
