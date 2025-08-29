package main

import (
	"github.com/labthrust/clusterprofile-credentials-plugins/pkg/core"
	provider "github.com/labthrust/clusterprofile-credentials-plugins/pkg/providers/secretreader"
)

func main() {
	p, err := provider.NewDefault()
	if err != nil {
		panic(err)
	}
	core.Run(*p)
}
