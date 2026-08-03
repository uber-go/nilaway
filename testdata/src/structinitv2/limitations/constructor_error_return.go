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

// Regression guard: (value, error) discharge for the idiomatic `return &T{}, err` early return
// (a zero/partial struct paired with the just-checked error variable). The error-checked field
// dereferences below are now correct and carry no //want.
//
// Discharge is flow-sensitive on both sides: the supply side binds a return's field nilability only
// when the error is definitely nil, so a non-nil-or-unknown error (including the `err` variable
// returned inside an `if err != nil` block) does not bind the zero struct's nil fields; the caller
// side still flags a caller that does not check the error.

type ctorErrInner struct{ x int }
type ctorErrVerifyRequest struct {
	req       *ctorErrInner
	parsedURL *ctorErrInner
}

func ctorErrMkID() (int, error) { return 0, nil }

// ctorErrBuild's early return pairs a zero struct with the error variable `err`. The error is not
// definitely nil, so the omitted fields must not be bound into the success-path summary.
func ctorErrBuild(r, p *ctorErrInner) (*ctorErrVerifyRequest, error) {
	_, err := ctorErrMkID()
	if err != nil {
		return &ctorErrVerifyRequest{}, err // zero struct + error variable: not bound
	}
	vr := &ctorErrVerifyRequest{req: r, parsedURL: p}
	return vr, nil
}

// ctorErrCaller returns on err!=nil, so both fields are set when the derefs run; not flagged.
func ctorErrCaller(r, p *ctorErrInner) {
	vr, err := ctorErrBuild(r, p)
	if err != nil {
		return
	}
	_ = vr.req.x       // safe — no //want
	_ = vr.parsedURL.x // safe — no //want
}

// ctorErrBuildConstructorErr pairs the zero struct with an error constructor (`errors.New`), which is
// definitely non-nil. Kept as a second regression case for the constructor path.
func ctorErrBuildConstructorErr(r, p *ctorErrInner) (*ctorErrVerifyRequest, error) {
	if r == nil {
		return &ctorErrVerifyRequest{}, errors.New("bad") // definitely-non-nil error: discharged
	}
	return &ctorErrVerifyRequest{req: r, parsedURL: p}, nil
}

func ctorErrCallerOK(r, p *ctorErrInner) {
	vr, err := ctorErrBuildConstructorErr(r, p)
	if err != nil {
		return
	}
	_ = vr.req.x // safe — no //want
}
