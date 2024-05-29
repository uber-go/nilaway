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

// Package abnormalflow checks code patterns with abnormal control flows (e.g., panic, log.Fatal
// etc.) that may lead to program terminations (such that any subsequent potential nil panics
// will not happen).
package abnormalflow

import (
	"errors"
	"log"
	"os"
	"runtime"
	"testing"
)

func testDirectDereference(msg string, t *testing.T, tb testing.TB) {
	var nilable *int
	switch msg {
	case "print":
		print("123")
		print(*nilable) //want "unassigned variable `nilable` dereferenced"
	case "panic":
		panic("foo")
		print(*nilable)
	case "log.Fatal":
		log.Fatal("foo")
		print(*nilable)
	case "log.Fatalf":
		log.Fatalf("foo")
		print(*nilable)
	case "os.Exit":
		os.Exit(1)
		print(*nilable)
	case "runtime.Goexit":
		runtime.Goexit()
		print(*nilable)
	case "testing.T.Fatal":
		t.Fatal("foo")
		print(*nilable)
	case "testing.T.Fatalf":
		t.Fatalf("foo")
		print(*nilable)
	case "testing.T.SkipNow":
		t.SkipNow()
		print(*nilable)
	case "testing.T.Skip":
		t.Skip()
		print(*nilable)
	case "testing.T.Skipf":
		t.Skipf("msg")
		print(*nilable)
	case "testing.TB.Fatal":
		tb.Fatal("foo")
		print(*nilable)
	case "testing.TB.Fatalf":
		tb.Fatalf("foo")
		print(*nilable)
	case "testing.TB.SkipNow":
		tb.SkipNow()
		print(*nilable)
	case "testing.TB.Skip":
		tb.Skip()
		print(*nilable)
	case "testing.TB.Skipf":
		tb.Skipf("msg")
		print(*nilable)
	}
}

func errReturn(a bool) (*int, error) {
	i := 42
	if a {
		return &i, nil
	}
	return nil, errors.New("some error")
}

func testErrReturn(msg string, b bool, t *testing.T, tb testing.TB) {
	ptr, err := errReturn(b)
	switch msg {
	case "print":
		if err != nil {
			print(err)
		}
		print(*ptr) //want "dereferenced"
	case "print_and_return":
		if err != nil {
			print(err)
			return
		}
		print(*ptr)
	case "panic":
		if err != nil {
			panic(err)
		}
		print(*ptr)
	case "log.Fatal":
		if err != nil {
			log.Fatal(err)
		}
		print(*ptr)
	case "log.Fatalf":
		if err != nil {
			log.Fatalf("msg %s", err)
		}
		print(*ptr)
	case "os.Exit":
		if err != nil {
			os.Exit(1)
		}
		print(*ptr)
	case "runtime.Goexit":
		if err != nil {
			runtime.Goexit()
		}
		print(*ptr)
	case "testing.T.Fatal":
		if err != nil {
			t.Fatal(err)
		}
		print(*ptr)
	case "testing.T.Fatalf":
		if err != nil {
			t.Fatalf("msg %s", err)
		}
		print(*ptr)
	case "testing.T.SkipNow":
		if err != nil {
			t.SkipNow()
		}
		print(*ptr)
	case "testing.T.Skip":
		if err != nil {
			t.Skip(err)
		}
		print(*ptr)
	case "testing.T.Skipf":
		if err != nil {
			t.Skipf("msg %s", err)
		}
		print(*ptr)
	case "testing.TB.Fatal":
		if err != nil {
			tb.Fatal(err)
		}
		print(*ptr)
	case "testing.TB.Fatalf":
		if err != nil {
			tb.Fatalf("msg %s", err)
		}
		print(*ptr)
	case "testing.TB.SkipNow":
		if err != nil {
			tb.SkipNow()
		}
		print(*ptr)
	case "testing.TB.Skip":
		if err != nil {
			tb.Skip(err)
		}
		print(*ptr)
	case "testing.TB.Skipf":
		if err != nil {
			tb.Skipf("msg %s", err)
		}
		print(*ptr)
	}
}