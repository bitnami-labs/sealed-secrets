// This binary is used to perform build automation duties.
//
// The existence of this file causes go mod to include dependencies used during
// development, such as code generation tools.
// However, the build tag below ensures these deps are not pulled during normal builds.

// +build tools

package main

import (
	"fmt"

	_ "k8s.io/code-generator"
)

func main() {
	fmt.Println("TODO")
}
