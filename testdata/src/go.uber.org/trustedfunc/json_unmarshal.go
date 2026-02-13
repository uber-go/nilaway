//  Copyright (c) 2026 Uber Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package trustedfunc

import (
	"encoding/json"
	"encoding/xml"
)

func jsonUnmarshalTest(c string) {
	switch c {
	case "json.Unmarshal - simple map":
		var outMap map[string]any
		if err := json.Unmarshal([]byte("{}"), &outMap); err != nil {
			return
		}
		_ = outMap["key"]
	case "json.Unmarshal - multiple checks":
		var outMap map[string]any
		var err error
		if err = json.Unmarshal([]byte("{}"), &outMap); err != nil {
			return
		}
		_ = outMap["key"]
	case "xml.Unmarshal - simple map":
		var outMap map[string]any
		if err := xml.Unmarshal([]byte("<root></root>"), &outMap); err != nil {
			return
		}
		_ = outMap["key"]
	}
}
