//	Copyright (c) 2023 Uber Technologies, Inc.
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

package diagnostic

import (
	"fmt"
	"go/token"
	"strings"

	"go.uber.org/nilaway/annotation"
)

type nilFlow struct {
	nilPath    []node // stores nil path of the flow from nilable source to conflict point
	nonnilPath []node // stores non-nil path of the flow from conflict point to dereference point
}

// addNilPathNode adds a new node to the nil path.
func (n *nilFlow) addNilPathNode(p annotation.Prestring, c annotation.Prestring) {
	nodeObj := newNode(p, c)

	// Note that in the implication graph, we traverse backwards from the point of conflict to the source of nilability.
	// Therefore, they are added in reverse order from what the program flow would look like. To account for this we
	// prepend the new node to nilPath because we want to print the program flow in its correct (forward) order.
	// TODO: instead of prepending here, we can reverse the nilPath slice while printing.
	n.nilPath = append([]node{nodeObj}, n.nilPath...)
}

// addNonNilPathNode adds a new node to the non-nil path
func (n *nilFlow) addNonNilPathNode(p annotation.Prestring, c annotation.Prestring) {
	nodeObj := newNode(p, c)
	n.nonnilPath = append(n.nonnilPath, nodeObj)
}

// String converts a nilFlow to a string representation, where each entry is the flow of the form: `<pos>: <reason>`
func (n *nilFlow) String() string {
	var allNodes []node
	allNodes = append(allNodes, n.nilPath...)
	allNodes = append(allNodes, n.nonnilPath...)

	var flow []string
	for _, nodeObj := range allNodes {
		flow = append(flow, nodeObj.String())
	}
	return "\n" + strings.Join(flow, "\n")
}

type node struct {
	producerPosition token.Position
	consumerPosition token.Position
	producerRepr     string
	consumerRepr     string
}

// newNode creates a new node object from the given producer and consumer Prestrings.
// LocatedPrestring contains accurate information about the position and the reason why NilAway deemed that position
// to be nilable. We use it if available, else we use the raw string representation available from the Prestring.
func newNode(p annotation.Prestring, c annotation.Prestring) node {
	nodeObj := node{}

	// get producer representation string
	if l, ok := p.(annotation.LocatedPrestring); ok {
		nodeObj.producerPosition = l.Location
		nodeObj.producerRepr = l.Contained.String()
	} else if p != nil {
		nodeObj.producerRepr = p.String()
	}

	// get consumer representation string
	if l, ok := c.(annotation.LocatedPrestring); ok {
		nodeObj.consumerPosition = l.Location
		nodeObj.consumerRepr = l.Contained.String()
	} else if c != nil {
		nodeObj.consumerRepr = c.String()
	}

	return nodeObj
}

func (n *node) String() string {
	posStr := "<no pos info>"
	reasonStr := ""
	if n.consumerPosition.IsValid() {
		posStr = n.consumerPosition.String()
	}

	if len(n.producerRepr) > 0 {
		reasonStr += n.producerRepr
	}
	if len(n.consumerRepr) > 0 {
		if len(n.producerRepr) > 0 {
			reasonStr += " "
		}
		reasonStr += n.consumerRepr
	}

	return fmt.Sprintf("\t- %s: %s", posStr, reasonStr)
}

func pathString(nodes []node) string {
	path := ""
	for _, n := range nodes {
		path += n.String()
	}
	return path
}
