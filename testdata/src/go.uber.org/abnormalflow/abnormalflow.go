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

func testDirectDereference(msg string, t *testing.T, b *testing.B, f *testing.F, tb testing.TB) {
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
	case "testing.B.Fatal":
		b.Fatal("foo")
		print(*nilable)
	case "testing.B.Fatalf":
		b.Fatalf("foo")
		print(*nilable)
	case "testing.B.SkipNow":
		b.SkipNow()
		print(*nilable)
	case "testing.B.Skip":
		b.Skip()
		print(*nilable)
	case "testing.B.Skipf":
		b.Skipf("msg")
		print(*nilable)
	case "testing.TB.Fatal":
		tb.Fatal("foo")
		print(*nilable) //want "unassigned variable `nilable` dereferenced"
	case "testing.TB.Fatalf":
		tb.Fatalf("foo")
		print(*nilable) //want "unassigned variable `nilable` dereferenced"
	case "testing.TB.SkipNow":
		tb.SkipNow()
		print(*nilable) //want "unassigned variable `nilable` dereferenced"
	case "testing.TB.Skip":
		tb.Skip()
		print(*nilable) //want "unassigned variable `nilable` dereferenced"
	case "testing.TB.Skipf":
		tb.Skipf("msg")
		print(*nilable) //want "unassigned variable `nilable` dereferenced"
	case "testing.F.Fatal":
		f.Fatal("foo")
		print(*nilable)
	case "testing.F.Fatalf":
		f.Fatalf("foo")
		print(*nilable)
	case "testing.F.SkipNow":
		f.SkipNow()
		print(*nilable)
	case "testing.F.Skip":
		f.Skip()
		print(*nilable)
	case "testing.F.Skipf":
		f.Skipf("msg")
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

func testErrReturn(msg string, val bool, t *testing.T, b *testing.B, f *testing.F, tb testing.TB) {
	ptr, err := errReturn(val)
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
	case "testing.B.Fatal":
		if err != nil {
			b.Fatal(err)
		}
		print(*ptr)
	case "testing.B.Fatalf":
		if err != nil {
			b.Fatalf("msg %s", err)
		}
		print(*ptr)
	case "testing.B.SkipNow":
		if err != nil {
			b.SkipNow()
		}
		print(*ptr)
	case "testing.B.Skip":
		if err != nil {
			b.Skip(err)
		}
		print(*ptr)
	case "testing.B.Skipf":
		if err != nil {
			b.Skipf("msg %s", err)
		}
		print(*ptr)
	case "testing.F.Fatal":
		if err != nil {
			f.Fatal(err)
		}
		print(*ptr)
	case "testing.F.Fatalf":
		if err != nil {
			f.Fatalf("msg %s", err)
		}
		print(*ptr)
	case "testing.F.SkipNow":
		if err != nil {
			f.SkipNow()
		}
		print(*ptr)
	case "testing.F.Skip":
		if err != nil {
			f.Skip(err)
		}
		print(*ptr)
	case "testing.F.Skipf":
		if err != nil {
			f.Skipf("msg %s", err)
		}
		print(*ptr)
	case "testing.TB.Fatal":
		if err != nil {
			tb.Fatal(err)
		}
		print(*ptr) //want "dereferenced"
	case "testing.TB.Fatalf":
		if err != nil {
			tb.Fatalf("msg %s", err)
		}
		print(*ptr) //want "dereferenced"
	case "testing.TB.SkipNow":
		if err != nil {
			tb.SkipNow()
		}
		print(*ptr) //want "dereferenced"
	case "testing.TB.Skip":
		if err != nil {
			tb.Skip(err)
		}
		print(*ptr) //want "dereferenced"
	case "testing.TB.Skipf":
		if err != nil {
			tb.Skipf("msg %s", err)
		}
		print(*ptr) //want "dereferenced"
	}
}
