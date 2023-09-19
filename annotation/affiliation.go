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

package annotation

import (
	"go/ast"
	"go/types"

	"go.uber.org/nilaway/util"
)

// This file contains annotation-embdded obo the affiliations mechanism

// An AffiliationPair is the atomic object of the affiliations mechanism:
// a pair consisting of an interface method and an implementing method
type AffiliationPair struct {
	ImplementingMethod *types.Func
	InterfaceMethod    *types.Func
}

// the only reason it's ok to create new ast.ident's here without going through
// GetDeclaringIdent is that they're entered directly into a FullTrigger in their usages.
// DO NOT DO THIS to create any expressions that will enter unmatched triggers

func (a AffiliationPair) interfaceMethodAsExpr() ast.Expr {
	return &ast.Ident{
		NamePos: a.InterfaceMethod.Pos(),
		Name:    a.InterfaceMethod.Name(),
		Obj:     nil,
	}
}

func (a AffiliationPair) implementingMethodAsExpr() ast.Expr {
	return &ast.Ident{
		NamePos: a.ImplementingMethod.Pos(),
		Name:    a.ImplementingMethod.Name(),
		Obj:     nil,
	}
}

// FullTriggerForInterfaceParamFlow takes the knowledge that `affiliation` represents an affiliation
// discovered in the analyzed code - for example, an assignment of a variable of interface type `I`
// to a value of pointer type `*S` - and returns a FullTrigger representing the assertion that
// the interface method can have a nilable parameter at position `paramNum` only if the implementing
// method has such a nilable parameter. This encodes "contravariance" of annotations for parameters.
// Precondition: paramNum < numParams(affiliation.InterfaceMethod)
func FullTriggerForInterfaceParamFlow(affiliation AffiliationPair, paramNum int) FullTrigger {
	return FullTrigger{
		Producer: &ProduceTrigger{
			Annotation: &InterfaceParamReachesImplementation{
				TriggerIfNilable: &TriggerIfNilable{
					Ann: ParamKeyFromArgNum(affiliation.InterfaceMethod, paramNum)},
				AffiliationPair: &affiliation,
			},
			Expr: affiliation.interfaceMethodAsExpr(),
		},
		Consumer: &ConsumeTrigger{
			Annotation: &MethodParamFromInterface{
				TriggerIfNonNil: &TriggerIfNonNil{
					Ann: ParamKeyFromArgNum(affiliation.ImplementingMethod, paramNum)},
				AffiliationPair: &affiliation,
			},
			Expr:         affiliation.implementingMethodAsExpr(),
			Guards:       util.NoGuards(),
			GuardMatched: false,
		},
	}
}

// FullTriggerForInterfaceResultFlow takes the knowledge that `affiliation` represents an affiliation
// discovered in the analyzed code - for example, an assignment of a variable of interface type `I`
// to a value of pointer type `*S` - and returns a FullTrigger representing the assertion that
// the implementing method can have a nilable result at position `retNum` only if the interface method
// has such a nilable result. This encodes "covariance" of annotations for results.
// Precondition: retNum < numResults(affiliation.InterfaceMethod)
func FullTriggerForInterfaceResultFlow(affiliation AffiliationPair, retNum int) FullTrigger {
	return FullTrigger{
		Producer: &ProduceTrigger{
			Annotation: &MethodResultReachesInterface{
				TriggerIfNilable: &TriggerIfNilable{
					Ann: RetKeyFromRetNum(affiliation.ImplementingMethod, retNum)},
				AffiliationPair: &affiliation,
			},
			Expr: affiliation.implementingMethodAsExpr(),
		},
		Consumer: &ConsumeTrigger{
			Annotation: &InterfaceResultFromImplementation{
				TriggerIfNonNil: &TriggerIfNonNil{
					Ann: RetKeyFromRetNum(affiliation.InterfaceMethod, retNum)},
				AffiliationPair: &affiliation,
			},
			Expr:         affiliation.interfaceMethodAsExpr(),
			Guards:       util.NoGuards(),
			GuardMatched: false,
		},
	}
}
