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

// The main purpose of this plugin is to encourage us to follow standard practices when writing notes as:
//
//	"MARKER(uid): note body"
//
// which is more readable and maintainable since it is easy to search and filter notes by MARKER and uid.
// more details can be found at https://pkg.go.dev/go/doc#Note
package notecheck
