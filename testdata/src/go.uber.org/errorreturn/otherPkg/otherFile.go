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

package otherPkg

import (
	"errors"
	"fmt"
)

func RetErr() error {
	return errors.New("some exported error message")
}

// Wrapf is an error wrapper defined in a separate package, used to test that the error wrapper
// heuristic works across package boundaries (via the facts mechanism).
func Wrapf(e error) error {
	if e == nil {
		return nil
	}
	return fmt.Errorf("wrapped: %w", e)
}

var GlobalErrorFromOtherPkg = fmt.Errorf("some other exported error message")
