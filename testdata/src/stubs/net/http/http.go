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

// Package http is a minimal stub of `net/http`, limited to the surface that the
// ErrorReturnNonnilResult hook test exercises. It is kept under testdata/src/stubs so that the
// hook's `^(stubs/)?net/http\.Client$` regex matches it, while the analyzed test package imports
// it as an external (out-of-scope) dependency -- mirroring how real user code consumes `net/http`.
// The stub intentionally does NOT model the documented contract itself; that is the hook's job.
package http

// Request is a minimal stub of net/http.Request.
type Request struct{}

// Response is a minimal stub of net/http.Response. Its pointer is the result type of Client.Do,
// and is nilable by default (the hook supplies the non-nil-on-nil-error guarantee).
type Response struct {
	StatusCode int
	Status     string
	Header     map[string][]string
	Body       *Body
}

// Body is a minimal stub of net/http's read/closable body.
type Body struct{}

func (b *Body) Close() error { return nil }

// Client is a minimal stub of net/http.Client. Its Do method has the same signature shape as the
// real net/http.Client.Do: it returns (*Response, error). The body is opaque so the analyzer treats
// the result as nilable by default, exactly like the unanalyzed stdlib method.
type Client struct{}

func (c *Client) Do(req *Request) (*Response, error) {
	return nil, nil
}
