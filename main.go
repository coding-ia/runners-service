package main

import (
	"context"
	"fmt"
	sdk "github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"log"
	"os"
)

func createNewConfig(socketPath string) sdk.Config {
	dir, _ := os.Getwd()
	fmt.Println(dir)
	kernelImage := "/home/brian/custom/vmlinux-5.10.223"

	var vcpuCount int64 = 1
	var memSizeMib int64 = 1024
	smt := false

	driveID := "root"
	isRootDevice := true
	isReadOnly := false
	pathOnHost := "/home/brian/custom/ubuntu-22.04.ext4"

	cfg := sdk.Config{
		SocketPath:      socketPath,
		KernelArgs:      "console=ttyS0 loglevel=3 reboot=k panic=1 pci=off selinux=0",
		KernelImagePath: kernelImage,
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  &vcpuCount,
			MemSizeMib: &memSizeMib,
			Smt:        &smt,
		},
		Drives: []models.Drive{
			{
				DriveID:      &driveID,
				IsRootDevice: &isRootDevice,
				IsReadOnly:   &isReadOnly,
				PathOnHost:   &pathOnHost,
			},
		},
	}

	return cfg
}

func main() {
	fcSocket := "/tmp/firecracker.socket"
	ctx := context.TODO()

	networkInterface := sdk.NetworkInterface{
		CNIConfiguration: &sdk.CNIConfiguration{
			NetworkName: "fcnet",
			IfName:      "veth0",
			BinPath:     []string{"/opt/cni/bin"},
			VMIfName:    "eth0",
		},
	}

	vmConfig := createNewConfig(fcSocket)
	vmConfig.NetworkInterfaces = append(vmConfig.NetworkInterfaces, networkInterface)

	cmd := sdk.VMCommandBuilder{}.WithSocketPath(fcSocket).WithBin("/home/brian/firecracker/firecracker").Build(ctx)
	m, err := sdk.NewMachine(ctx, vmConfig, sdk.WithProcessRunner(cmd))

	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(fcSocket)

	err = m.Start(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := m.StopVMM(); err != nil {
			log.Fatal(err)
		}
	}()
	defer func() {
		if err := m.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}()

	vmIP := m.Cfg.NetworkInterfaces[0].StaticConfiguration.IPConfiguration.IPAddr.IP.String()
	fmt.Printf("IP of VM: %v\n", vmIP)

	for _, vd := range m.Cfg.VsockDevices {
		fmt.Printf("ID: %s\n", vd.ID)
		fmt.Printf("Path: %s\n", vd.Path)
	}

	for {
	}
}
