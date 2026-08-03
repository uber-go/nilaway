//  Copyright (c) 2023 Uber Technologies, Inc.
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

package limitations

// False positive: a field's nil producer is not cleared across two guards correlated by the same
// predicate. The analysis admits the infeasible path where the assignment block (gated on
// `Source == nil && caller != ""`) is skipped while the deref block (gated on `caller != ""`) runs.
// The zero-value allocation `var request corrReq` is load-bearing: it gives Source a definitely-nil
// producer that a bare parameter would not.

type corrSource struct{ Caller *string }
type corrReq struct{ Source *corrSource }

// corrCorrelatedGuards is the false positive: whenever the `caller != ""` deref block runs, the
// earlier block has already set Source non-nil, but the two predicates are not tied together.
func corrCorrelatedGuards(caller string) corrReq {
	var request corrReq // zero value: request.Source is definitely nil
	if request.Source == nil && caller != "" {
		request.Source = &corrSource{}
	}
	if caller != "" {
		request.Source.Caller = &caller //want "uninitialized field `Source` accessed field `Caller`"
	}
	return request
}
