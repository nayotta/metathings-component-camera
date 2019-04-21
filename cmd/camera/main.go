package main

import (
	"os"

	component "github.com/nayotta/metathings/pkg/component"
)

func main() {
	mdl, err := component.NewModule(os.Args[0], nil)
	if err != nil {
		panic(err)
	}
	err = mdl.Launch()
	if err != nil {
		panic(err)
	}
}
