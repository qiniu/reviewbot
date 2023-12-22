package main

import (
	"fmt"
	"testing"
)

func TestParsePatch(t *testing.T) {
	patch := "@@ -132,7 +132,7 @@ module Test @@ -1000,7 +1000,7 @@ module Test"
	fmt.Println(ParsePatch(patch))
}
