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

// Package internal is a minimal stub of `go.uber.org/cadence/internal`, which is where the `Future`
// interface (re-exported as `go.uber.org/cadence/workflow.Future`) is actually declared.
package internal

// Context is a minimal stub of cadence's workflow context.
type Context interface {
	Value(key interface{}) interface{}
}

// Future is a minimal stub of cadence's `Future` interface. Its `Get` method blocks until the
// future is ready and, on success (a nil error return), populates the value pointed to by
// `valuePtr` (passed by address as `&v`).
type Future interface {
	Get(ctx Context, valuePtr interface{}) error
	IsReady() bool
}
