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

// newContractVal converts a keyword string into the corresponding function ContractVal.
func newContractVal(keyword string) ContractVal {
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

// Contract represents a function contract.
type Contract struct {
	// Ins is the list of input contract values, where the index is the index of the parameter.
	Ins []ContractVal
	// Outs is the list of output contract values, where the index is the index of the return.
	Outs []ContractVal
}
