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

// Regression guard: the decode_localvar_field case, shown to survive an address-of-value-struct
// indirection and a depth-2 receiver. A field set from a local variable holding a non-nil value (or
// a function result) is resolved against the variable's real value, so the deref below carries no
// //want.

type addrOfTimeFilter struct{ v int }
type addrOfFilterRuleSet struct{ TimeFilter *addrOfTimeFilter }
type addrOfCardMetadata struct{ FilterRules *addrOfFilterRuleSet }
type addrOfInboxCard struct{ Metadata *addrOfCardMetadata }

// addrOfMakeRuleSet always returns a non-nil set.
func addrOfMakeRuleSet() *addrOfFilterRuleSet {
	var rs addrOfFilterRuleSet
	return &rs // unconditionally non-nil
}

// addrOfMap: FilterRules is provably non-nil (addrOfMakeRuleSet's result), so the receiver chain
// cannot be nil. (Reading the nilable .TimeFilter is safe; it is returned, not dereferenced.)
func addrOfMap(inboxCard *addrOfInboxCard) *addrOfTimeFilter {
	filterRules := addrOfMakeRuleSet()                       // non-nil
	cardMeta := addrOfCardMetadata{FilterRules: filterRules} // field set from a local variable
	inboxCard.Metadata = &cardMeta
	return inboxCard.Metadata.FilterRules.TimeFilter // safe — no //want
}
