//  Copyright (c) 2023 Uber Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package lazyinit

type U struct {
	Y int
}

type X struct {
	F *U
}

func (x *X) GetF() *U { return x.F }

func getterGuard(x *X) {
	if x.GetF() == nil {
		x.F = &U{}
	}
	x.F.Y = 1
}

func fieldGuard(x *X) {
	if x.F == nil {
		x.F = &U{}
	}
	x.F.Y = 1
}

func twoPaths(x *X, c bool) int {
	x = &X{}
	if x.F == nil {
		if c {
			x.F = &U{}
		}
	}
	return x.F.Y //want "accessed field `Y`"
}

func guardOnly(x *X, c bool) int {
	x = &X{}
	if x.F == nil {
		_ = c
	}
	return x.F.Y //want "accessed field `Y`"
}

var noop = &U{}

func singleton(x *X) {
	if x.F == nil {
		x.F = noop
	}
	x.F.Y = 1
}

func postLiteral(x *X) {
	x.F = &U{}
	x.F.Y = 1
}

func maybeNil() *U { return nil }

func maybeX() *X { return nil }

func unknownStore(x *X) {
	x = &X{}
	x.F = maybeNil()
	x.F.Y = 1 //want "accessed field `Y`"
}

func renil(x *X) {
	x = &X{}
	x.F = &U{}
	x.F = nil
	x.F.Y = 1 //want "accessed field `Y`"
}

func nilBase(x *X) {
	x = maybeX()
	x.F = &U{} //want "accessed field `F`"
	x.F.Y = 1  //want "accessed field `F`"
}

func uncovered(x *X, cond bool) {
	x = &X{}
	if cond {
		x.F = &U{}
	}
	x.F.Y = 1 //want "accessed field `Y`"
}
