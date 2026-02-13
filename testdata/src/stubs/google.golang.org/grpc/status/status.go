//  Copyright (c) 2024 Uber Technologies, Inc.
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

// <nilaway no inference>
package status

import "stubs/google.golang.org/grpc/codes"

// Error returns an error representing c and msg. If c is OK, returns nil.
func Error(c codes.Code, msg string) error {
	return nil
}

// Errorf returns an error representing c and a formatted msg. If c is OK, returns nil.
func Errorf(c codes.Code, format string, a ...interface{}) error {
	return nil
}
