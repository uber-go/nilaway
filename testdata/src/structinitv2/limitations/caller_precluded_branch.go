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

// False positive: context-insensitive return merge of a caller-precluded branch. A constructor
// returns a struct with an omitted field only on a branch that the sole caller's precondition makes
// dead, but the field is flagged anyway.
//
// The return summary is context-insensitive: one site per (function, result index, field path),
// merged over every return statement with no relation to the caller's arguments. The dead branch's
// nil field therefore poisons the merged summary. There is no error result here, so this is purely a
// cross-function precondition the analysis cannot correlate.

type precludedCriteria struct{ ReturnSelfIfRoot bool }
type precludedRequest struct{ Criteria *precludedCriteria }
type precludedThriftReq struct{ x int }

// precludedMap returns an empty request on nil input, else a fully populated one. The Criteria-nil
// return is reachable only when thriftReq == nil.
func precludedMap(thriftReq *precludedThriftReq) *precludedRequest {
	if thriftReq == nil {
		return &precludedRequest{} // Criteria nil — dead for callers that pass a non-nil thriftReq
	}
	return &precludedRequest{Criteria: &precludedCriteria{ReturnSelfIfRoot: true}}
}

// precludedCaller is the false positive: it guards `args != nil`, so precludedMap always takes the
// populated branch — but the merged summary says nilable.
func precludedCaller(args *precludedThriftReq) bool {
	if args == nil {
		return false
	}
	protoReq := precludedMap(args)            // args is non-nil here, so Criteria is set
	return protoReq.Criteria.ReturnSelfIfRoot //want "field `Criteria` of result 0 of `precludedMap` accessed field `ReturnSelfIfRoot`"
}

// precludedMapAlwaysSet is the negative control: it sets Criteria on every return path, so the
// summary is unconditionally non-nil. This isolates the dead nil-input branch as the source of the
// false positive.
func precludedMapAlwaysSet(thriftReq *precludedThriftReq) *precludedRequest {
	if thriftReq == nil {
		return &precludedRequest{Criteria: &precludedCriteria{}}
	}
	return &precludedRequest{Criteria: &precludedCriteria{ReturnSelfIfRoot: true}}
}

func precludedCallerOK(args *precludedThriftReq) bool {
	protoReq := precludedMapAlwaysSet(args)
	return protoReq.Criteria.ReturnSelfIfRoot // safe — no //want
}
