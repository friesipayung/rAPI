// Command version prints the application version (used by CI and humans).
package main

import (
	"fmt"

	"github.com/friesipayung/rapi/internal/version"
)

func main() {
	fmt.Println(version.Version)
}
