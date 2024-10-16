package hack

import (
	"context"
	"github.com/firecracker-microvm/firecracker-go-sdk"
)

const (
	CreateNetworkInterfacesJailerHandlerName = "fcinit.CreateNetworkInterfaces"
)

type JailerMachine struct {
	m firecracker.Machine
}

var CreateNetworkInterfacesHandler = firecracker.Handler{
	Name: CreateNetworkInterfacesJailerHandlerName,
	Fn: func(ctx context.Context, m *firecracker.Machine) error {
		return nil
	},
}
