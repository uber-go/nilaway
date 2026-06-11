//  Copyright (c) 2025 Uber Technologies, Inc.
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

package inference

import "os"

// osGlobalsTest verifies that reads of the well-known stdlib globals os.Stdout/Stderr/Stdin and
// os.Args are treated as non-nil, so dereferencing/indexing them is not flagged. Without this
// support, NilAway infers these as nilable (their initializers can technically return nil), which
// produces a large class of false positives on idiomatic code.
func osGlobalsTest(c string, p []byte) {
	switch c {
	case "os.Stdout":
		// Method call dereferences the receiver; os.Stdout is non-nil.
		_, _ = os.Stdout.Write(p)
	case "os.Stderr":
		_, _ = os.Stderr.Write(p)
	case "os.Stdin":
		_, _ = os.Stdin.Read(p)
	case "os.Args":
		// Indexing/ranging dereferences the slice; os.Args is non-nil.
		print(os.Args[0])
		for _, a := range os.Args {
			print(a)
		}
	}
}

// osGlobalsFlowTest verifies the non-nil assumption also propagates when the global is assigned to a
// local and then consumed.
func osGlobalsFlowTest(p []byte) {
	f := os.Stdout
	_, _ = f.Write(p)

	args := os.Args
	print(args[0])
}
