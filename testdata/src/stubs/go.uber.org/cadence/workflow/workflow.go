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

// Package workflow is a minimal stub of `go.uber.org/cadence/workflow`. Like the real package, it
// re-exports the `Future` and `Context` types from the internal package via type aliases, so a call
// to `f.Get(ctx, &v)` resolves to the method declared in `go.uber.org/cadence/internal`.
package workflow

import "stubs/go.uber.org/cadence/internal"

// Context aliases internal.Context, mirroring the real cadence package.
type Context = internal.Context

// Future aliases internal.Future, mirroring the real cadence package.
type Future = internal.Future
