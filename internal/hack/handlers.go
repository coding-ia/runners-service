package hack

import (
	"context"
	"github.com/firecracker-microvm/firecracker-go-sdk"
)

const (
	SetupJailerNetworkHandlerName = "fcinit.SetupNetwork"
)

type Jailer struct {
	networkInterfaces firecracker.NetworkInterfaces
}

var SetupJailerNetworkHandler = firecracker.Handler{
	Name: SetupJailerNetworkHandlerName,
	Fn: func(ctx context.Context, m *firecracker.Machine) error {
		return nil
	},
}
