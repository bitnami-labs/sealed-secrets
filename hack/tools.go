// This file forces go mod to include dependencies used during build, such as
// code generation tools.
// The build tag below ensures this dep is not pulled during normal builds.

//go:build tools
// +build tools

package tools

import (
	_ "k8s.io/code-generator"
)
