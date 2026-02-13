package main

import (
	xLog "github.com/bamboo-services/bamboo-base-go/log"
	xMain "github.com/bamboo-services/bamboo-base-go/main"
	xReg "github.com/bamboo-services/bamboo-base-go/register"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/app/route"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/app/startup"
)

func main() {
	reg := xReg.Register(startup.Init())
	log := xLog.WithName(xLog.NamedMAIN)

	xMain.Runner(reg, log, route.NewRoute, nil)
	return
}
