//  Copyright (c) 2024 Uber Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package middle

import "go.uber.org/factexport/upstream"

func NewEvent() upstream.ExportedEvent {
	return upstream.ExportedEvent{}
}

func NewBox() upstream.Box[int] {
	return upstream.Box[int]{}
}

func NewVisible() upstream.Visible {
	return upstream.Visible{}
}
