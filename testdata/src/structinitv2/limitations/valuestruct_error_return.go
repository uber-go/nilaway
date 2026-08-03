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

import "errors"

// Regression guard: the (value, error) discharge of constructor_error_return.go, for a value struct
// whose interface field's method is invoked across a further parameter boundary. The failure path's
// zero `valErrParams{}` does not poison the success-path summary, so the non-nil Keysrv flows through
// the boundary and the call below carries no //want.

type valErrKeyService interface{ IssuerKeySpace(s string) int }
type valErrParams struct{ Keysrv valErrKeyService }

type valErrRealKS struct{}

func (valErrRealKS) IssuerKeySpace(string) int { return 0 }

func valErrGetKeySources(fail bool) (valErrKeyService, error) {
	if fail {
		return nil, errors.New("boom")
	}
	return valErrRealKS{}, nil
}

// valErrGetParams's failure return pairs the zero struct with the error variable `err`, which is not
// definitely nil, so the omitted Keysrv field must not be bound into the success-path summary.
func valErrGetParams(fail bool) (valErrParams, error) {
	ks, err := valErrGetKeySources(fail)
	if err != nil {
		return valErrParams{}, err // zero value struct + error variable: not bound
	}
	return valErrParams{Keysrv: ks}, nil
}

// valErrNewAuthSSI: on every call reaching here Keysrv was set on valErrGetParams's success path.
func valErrNewAuthSSI(p valErrParams) int {
	return p.Keysrv.IssuerKeySpace("x") // safe — no //want
}

func valErrProvide(fail bool) int {
	params, err := valErrGetParams(fail)
	if err != nil {
		return -1
	}
	return valErrNewAuthSSI(params)
}

// valErrGetParamsCtor pairs the zero struct with an error constructor (`errors.New`), which is
// definitely non-nil. Kept as a second regression case for the constructor path.
func valErrGetParamsCtor(fail bool) (valErrParams, error) {
	if fail {
		return valErrParams{}, errors.New("boom") // definitely-non-nil error: discharged
	}
	return valErrParams{Keysrv: valErrRealKS{}}, nil
}

func valErrUseCtor(p valErrParams) int {
	return p.Keysrv.IssuerKeySpace("y") // safe — no //want
}

func valErrProvideOK(fail bool) int {
	params, err := valErrGetParamsCtor(fail)
	if err != nil {
		return -1
	}
	return valErrUseCtor(params)
}
