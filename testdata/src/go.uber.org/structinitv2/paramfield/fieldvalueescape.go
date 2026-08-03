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

// Reading a parameter field as a value that escapes the reading function: `return b.inner` reads
// field `inner` without dereferencing it, so only the selector's value demand (not the base demand
// of a deeper selector) puts `inner` in the parameter's read set. The dereference then happens at
// the caller, on the escaped value; without value demand no boundary consumer exists and the nil
// flow is silently dropped.

package paramfield

type escapeLeaf struct {
	ptr *int
}

type escapeBox struct {
	inner *escapeLeaf
}

// The field value escapes by being returned directly.
func escapeDirect(b *escapeBox) *escapeLeaf {
	return b.inner
}

func useEscapeDirectUnsafe() {
	b := &escapeBox{}
	// Temporarily silent: `return b.inner` is a whole-result param source, so the result is
	// severed from the shared return summary (which would merge this nil flow into
	// useEscapeDirectSafe below). Flips back to a per-call report when call-site resolution
	// lands.
	print(escapeDirect(b).ptr)
}

func useEscapeDirectSafe() {
	b := &escapeBox{inner: &escapeLeaf{}}
	print(escapeDirect(b).ptr)
}

// The field value escapes through a local snapshot before being returned.
func escapeSnapshot(b *escapeBox) *escapeLeaf {
	w := b.inner
	return w
}

func useEscapeSnapshotUnsafe() {
	b := &escapeBox{}
	print(escapeSnapshot(b).ptr) //want "uninitialized field `inner`"
}

func useEscapeSnapshotSafe() {
	b := &escapeBox{inner: &escapeLeaf{}}
	print(escapeSnapshot(b).ptr)
}

// The same value demand applies to a method receiver boundary. No safe negative control here:
// before the param-source sever, the method's shared return site merged all callers, so a safe
// caller of innerValue would have inherited this caller's nil flow.
func (b *escapeBox) innerValue() *escapeLeaf {
	return b.inner
}

func useReceiverEscapeUnsafe() {
	b := &escapeBox{}
	// Temporarily silent: receiver-projection param sources sever the result from the shared summary;
	// flips back to a per-call report when call-site resolution lands.
	print(b.innerValue().ptr)
}
