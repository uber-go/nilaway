package functioncontracts

import (
	"fmt"
	"go/ast"
	"go/types"
	"regexp"
	"strings"

	"go.uber.org/nilaway/config"
	"go.uber.org/nilaway/util"
	"golang.org/x/tools/go/analysis"
)

// ContractVal represents the possible value appearing in a function contract.
type ContractVal string

const (
	// NonNil has keyword "nonnil".
	NonNil ContractVal = "nonnil"
	// False has keyword "false".
	False ContractVal = "false"
	// True has keyword "true".
	True ContractVal = "true"
	// Any has keyword "_".
	Any ContractVal = "_"
)

// stringToContractVal converts a keyword string into the corresponding function ContractVal.
func stringToContractVal(keyword string) ContractVal {
	switch keyword {
	case "nonnil":
		return NonNil
	case "false":
		return False
	case "true":
		return True
	case "_":
		return Any
	default:
		// TODO: The ideal way to handle this is to keep track of this contract parsing error and
		//  move on to the other contracts. But this may also require some refactoring of other
		//  parts (we do not currently handle partial recoveries anyways)
		panic("Unexpected keyword for ContractVal: " + keyword)
	}
}

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

// FunctionContract represents a function contract.
type FunctionContract struct {
	Ins  []ContractVal
	Outs []ContractVal
}

// Map stores the mappings from *types.Func to associated function contracts.
type Map map[*types.Func][]*FunctionContract

// collectFunctionContracts collects all the function contracts and returns a map that associates
// every function with its contracts if it has any. One function can have multiple contracts.
func collectFunctionContracts(pass *analysis.Pass) Map {
	conf := pass.ResultOf[config.Analyzer].(*config.Config)

	m := Map{}
	for _, file := range pass.Files {
		if !conf.IsFileInScope(file) || !util.DocContainsFunctionContractsCheck(file.Doc) {
			continue
		}
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok || funcDecl.Doc == nil {
				// Ignore any non-function declaration or the function has no comment
				// TODO: If we want to support contracts for anonymous function (function
				//  literals) in the future, then we need to handle more types here.
				continue
			}
			funcObj := pass.TypesInfo.ObjectOf(funcDecl.Name).(*types.Func)
			if funcContracts := parseContractsForSingleFunction(funcDecl.Doc); len(funcContracts) != 0 {
				m[funcObj] = funcContracts
			}
		}
	}
	return m
}

// parseContractsForSingleFunction parses a slice of function contracts from a singe comment group.
// If no contract is found from the comment group, an empty slice is returned.
func parseContractsForSingleFunction(doc *ast.CommentGroup) []*FunctionContract {
	contracts := make([]*FunctionContract, 0)
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
