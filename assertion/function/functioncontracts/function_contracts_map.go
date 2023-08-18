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

// FunctionContract represents a function contract.
type FunctionContract struct {
	Ins  []ContractVal
	Outs []ContractVal
}

// Map stores the mappings from *types.Func.FullName() string to associated function contracts.
type Map map[string][]*FunctionContract

// newNonnilToNonnilContract creates a function contract that has only one NonNil value at the
// given index p for input and r for output.
func newNonnilToNonnilContract(p, nParams, r, nRets int) *FunctionContract {
	return &FunctionContract{
		Ins:  newSingleNonnilContractValList(p, nParams),
		Outs: newSingleNonnilContractValList(r, nRets),
	}
}

// newSingleNonnilContractValList creates a list of n ContractVals with only one NonNil value at
// the given index t.
func newSingleNonnilContractValList(t, n int) []ContractVal {
	vals := make([]ContractVal, n)
	for i := 0; i < n; i++ {
		if i == t {
			vals[i] = NonNil
		} else {
			vals[i] = Any
		}
	}
	return vals
}

// IsGeneralNonnnilToNonnil returns true if the given contract is a general nonnil to nonnil
// contract. Namely, the contract has only one nonnil in input and only one nonnil in output and
// all the other values are any, e.g., contract(_,nonnil->nonnil,_) is OK, but
// contract(nonnil,nonnil->nonnil,_) is not.
func (c *FunctionContract) IsGeneralNonnnilToNonnil() bool {
	return oneNonnilOthersAny(c.Ins) && oneNonnilOthersAny(c.Outs)
}

// IndexOfNonnilIn returns the index of the first NonNil value in the input of the contract. If
// there is no NonNil value in the input, it returns -1.
func (c *FunctionContract) IndexOfNonnilIn() int {
	for i, val := range c.Ins {
		if val == NonNil {
			return i
		}
	}
	return -1
}

// IndexOfNonnilOut returns the index of the first NonNil value in the output of the contract. If
// there is no NonNil value in the output, it returns -1.
func (c *FunctionContract) IndexOfNonnilOut() int {
	for i, val := range c.Outs {
		if val == NonNil {
			return i
		}
	}
	return -1
}

func oneNonnilOthersAny(vals []ContractVal) bool {
	seenNonnil := false
	for _, val := range vals {
		if val != Any && val != NonNil {
			return false
		}
		if val == Any {
			continue
		}
		// val == NonNil
		if seenNonnil {
			return false
		}
		seenNonnil = true
	}
	return seenNonnil
}
