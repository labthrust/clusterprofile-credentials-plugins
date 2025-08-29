package main

import (
	"github.com/labthrust/clusterprofile-credentials-plugins/pkg/core"
	provider "github.com/labthrust/clusterprofile-credentials-plugins/pkg/providers/eks"
)

func main() {
	core.Run(provider.Provider{})
}
