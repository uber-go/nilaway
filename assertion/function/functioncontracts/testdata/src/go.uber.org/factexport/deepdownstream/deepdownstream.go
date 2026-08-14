//  Copyright (c) 2024 Uber Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package deepdownstream

import "go.uber.org/factexport/middle"

func Use() bool {
	return middle.NewEvent().IsValid()
}
