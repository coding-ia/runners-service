package main

import (
	"context"
	"fmt"
	sdk "github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"log"
	"net"
	"os"
)

func boolPtr(b bool) *bool {
	return &b
}

func stringPtr(s string) *string {
	return &s
}

func createNewConfig(socketPath string) sdk.Config {
	dir, _ := os.Getwd()
	fmt.Println(dir)
	kernelImage := "/home/brian/custom/vmlinux-5.10.223"

	var vcpuCount int64 = 1
	var memSizeMib int64 = 1024
	smt := false

	driveID := "root"
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
				IsRootDevice: boolPtr(true),
				IsReadOnly:   boolPtr(false),
				PathOnHost:   &pathOnHost,
			},
			//{
			//	DriveID:      stringPtr("secondary"),
			//	IsRootDevice: boolPtr(false),
			//	IsReadOnly:   boolPtr(false),
			//	PathOnHost:   stringPtr("/home/brian/custom/overlay.ext4"),
			//},
		},
		MmdsAddress: net.ParseIP("169.254.169.254"),
		MmdsVersion: sdk.MMDSv2,
	}

	return cfg
}

func main() {
	fcSocket := "/tmp/firecracker.socket"
	ctx := context.Background()

	networkInterface := sdk.NetworkInterface{
		CNIConfiguration: &sdk.CNIConfiguration{
			NetworkName: "fcnet",
			IfName:      "veth0",
			BinPath:     []string{"/opt/cni/bin"},
		},
		AllowMMDS: true,
	}

	vmConfig := createNewConfig(fcSocket)
	vmConfig.NetworkInterfaces = append(vmConfig.NetworkInterfaces, networkInterface)

	cmd := sdk.VMCommandBuilder{}.
		WithBin("firecracker").
		WithSocketPath(fcSocket).
		WithStdin(os.Stdin).
		WithStdout(os.Stdout).
		WithStderr(os.Stderr).
		Build(ctx)

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

	metaDataIP := m.Cfg.MmdsAddress.String()
	fmt.Printf("Metadata IP: %v\n", metaDataIP)

	//metadata := map[string]string{
	//	"latest/meta-data/first-name":        "Brian",
	//	"latest/meta-data/last-name":         "Junker",
	//	"latest/meta-data/network/interface": "test",
	//}
	metadata := map[string]interface{}{
		"latest": map[string]interface{}{
			"meta-data": map[string]interface{}{
				"first-name": "John",
				"last-name":  "Doe",
			},
		},
	}

	m.SetMetadata(ctx, metadata)

	vmIP := m.Cfg.NetworkInterfaces[0].StaticConfiguration.IPConfiguration.IPAddr.IP.String()
	vmMAC := m.Cfg.NetworkInterfaces[0].StaticConfiguration.MacAddress
	fmt.Printf("MAC of VM: %v\n", vmMAC)
	fmt.Printf("IP of VM: %v\n", vmIP)

	for {
	}
}
