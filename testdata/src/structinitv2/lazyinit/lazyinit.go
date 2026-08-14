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

type misleadingX struct {
	F     *U
	Other *U
}

func (x *misleadingX) GetF() *U { return x.Other }

func misleadingGetter(x *misleadingX) {
	if x.GetF() == nil {
		x.F = &U{}
	}
	x.F.Y = 1 //want "accessed field `Y`"
}

func runMisleadingGetter() {
	misleadingGetter(&misleadingX{})
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

func Map(x *X) int {
	if x.GetF() == nil {
		x.F = &U{}
	}
	return x.F.Y
}

func Use() int {
	return Map(&X{})
}

type Request struct {
	ContentOptions *ContentOptions
}

func (r *Request) GetContentOptions() *ContentOptions { return r.ContentOptions }

type ContentOptions struct {
	Android *Android
}

type Android struct {
	Priority int
}

func pushRequest(req *Request) int {
	co := req.GetContentOptions()
	if co == nil {
		co = &ContentOptions{}
		req.ContentOptions = co
	}
	if co.Android == nil {
		co.Android = &Android{}
	}
	return co.Android.Priority
}

func requestCaller() int {
	return pushRequest(&Request{})
}

func negativeCaller(x *X) {
	if x.GetF() == nil {
		_ = 0
	}
	_ = x.F.Y //want "accessed field `Y`"
}

func runNegativeCaller() {
	negativeCaller(&X{})
}

func phiAlias(x *X, c bool) int {
	y := x
	if c {
		y = &X{}
	}
	if x.GetF() == nil {
		x.F = &U{}
	}
	y.F = nil
	return x.F.Y //want "accessed field `Y`"
}

func usePhiAlias() {
	phiAlias(&X{}, false)
}

func localPhiAlias(c bool) int {
	x := &X{}
	y := x
	if c {
		y = &X{}
	}
	if x.F == nil {
		x.F = &U{}
	}
	y.F = nil
	if !c {
		x.F = y.F
	}
	return x.F.Y //want "accessed field `Y`"
}

func useLocalPhiAlias() {
	localPhiAlias(false)
}

type H struct {
	P *X
}

func loadAlias(x *X, h *H) int {
	h.P = x
	if x.GetF() == nil {
		x.F = &U{}
	}
	h.P.F = nil
	return x.F.Y //want "accessed field `Y`"
}

func useLoadAlias() {
	loadAlias(&X{}, &H{})
}
