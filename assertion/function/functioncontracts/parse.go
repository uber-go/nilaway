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
func parseContracts(doc *ast.CommentGroup) []*FunctionContract {
	contracts := make([]*FunctionContract, 0)
	if doc == nil {
		return contracts
	}
	for _, lineComment := range doc.List {
		res := _contractRE.FindAllStringSubmatch(lineComment.Text, -1)
		if res == nil {
			continue
		}
		for _, matching := range res {
			// matching is a slice of three elements; the first is the whole matched string and the
			// next two are the captured groups of contract values before and after `->`.
			ins := parseListOfContractValues(matching[1])
			outs := parseListOfContractValues(matching[2])
			ctrt := &FunctionContract{
				Ins:  ins,
				Outs: outs,
			}
			contracts = append(contracts, ctrt)
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
		contractVals[i] = stringToContractVal(strings.TrimSpace(v))
	}
	return contractVals
}
