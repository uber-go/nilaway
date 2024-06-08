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

package functioncontracts

import (
	"fmt"
	"go/ast"
	"regexp"
	"strings"
)

const _sep = ","
const _contractKeyword = "contract"
const _contractValKeyword = NonNil + "|" + False + "|" + True + "|" + Any

// _contractRE matches multiple function contracts in the same line. Each contract looks like
// `contract(VALUE(,VALUE)+ -> VALUE(,VALUE)+)`. The RE also captures two lists of VALUEs,
// i.e., the part before and after `->` for each contract.
// Note that we match start and end of the line here and add a non-capturing group for the entire
// contract, and we disallow anything else except whitespace to appear between contracts, so we
// acknowledge only the contracts written in their own line.
var _contractRE = regexp.MustCompile(
	fmt.Sprintf("^\\s*//\\s*(?:\\s*%s\\s*\\(\\s*((?:%s)(?:\\s*,\\s*(?:%s))*)\\s*->\\s*((?:%s)(?:\\s*,\\s*(?:%s))*)\\s*\\)\\s*)+$",
		_contractKeyword, _contractValKeyword, _contractValKeyword, _contractValKeyword, _contractValKeyword))

// parseContracts parses a slice of function contracts from a singe comment group. If no contract
// is found from the comment group, an empty slice is returned.
func parseContracts(doc *ast.CommentGroup) Contracts {
	if doc == nil {
		return nil
	}

	var contracts Contracts
	for _, lineComment := range doc.List {
		for _, matching := range _contractRE.FindAllStringSubmatch(lineComment.Text, -1) {
			// matching is a slice of three elements; the first is the whole matched string and the
			// next two are the captured groups of contract values before and after `->`.
			ins := parseListOfContractValues(matching[1])
			outs := parseListOfContractValues(matching[2])
			contracts = append(contracts, Contract{
				Ins:  ins,
				Outs: outs,
			})
		}
	}
	return contracts
}

// parseListOfContractValues splits a string of comma separated contract value keywords and returns
// a slice of ContractVal.
func parseListOfContractValues(wholeStr string) []ContractVal {
	valKeywords := strings.Split(wholeStr, _sep)
	contractVals := make([]ContractVal, len(valKeywords))
	for i, v := range valKeywords {
		contractVals[i] = newContractVal(strings.TrimSpace(v))
	}
	return contractVals
}
