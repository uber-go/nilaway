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

import "stubs/net/http"

// testHTTPClientDo exercises the ErrorReturnNonnilResult hook for `(*http.Client).Do`: per the
// net/http package docs, a nil error return guarantees the returned `*http.Response` is non-nil.
// So once the error is checked to be nil, field accesses on the response must not be flagged.
//
// This imports the stub at stubs/net/http (matched by the hook's `^(stubs/)?net/http\.Client$`
// regex) rather than the real net/http, to keep the exported fact corpus small.

func testHTTPClientDo(client *http.Client, req *http.Request) {
	// `err != nil` early return: response is non-nil on the fallthrough (error-is-nil) path.
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	print(resp.StatusCode) // safe
	print(resp.Status)     // safe
	_ = resp.Body          // safe

	// `err == nil` positive check: response is non-nil inside the block.
	resp2, err2 := client.Do(req)
	if err2 == nil {
		print(resp2.StatusCode) // safe
	}

	// Error return not checked at all: no guarantee.
	resp3, _ := client.Do(req)
	print(resp3.StatusCode) //want "lacking guarding; accessed field `StatusCode`"

	// Dereference on the error path (`err != nil`): response is not guarded here.
	resp4, err4 := client.Do(req)
	if err4 != nil {
		print(resp4.StatusCode) //want "lacking guarding; accessed field `StatusCode`"
	}
}
