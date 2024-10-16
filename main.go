package main

import (
	"context"
	"fmt"
	sdk "github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

func boolPtr(b bool) *bool {
	return &b
}

func intPtr(i int) *int {
	return &i
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
	c := context.Background()
	ctx, cancel := context.WithCancel(c)
	defer cancel()

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

	/*
		cmd := sdk.VMCommandBuilder{}.
			WithBin("firecracker").
			WithSocketPath(fcSocket).
			WithStdin(os.Stdin).
			WithStdout(os.Stdout).
			WithStderr(os.Stderr).
			Build(ctx)
	*/

	vmConfig.JailerCfg = &sdk.JailerConfig{
		ID:             "551e7604-e35c-42b3-b825-416853441234",
		JailerBinary:   "jailer",
		ExecFile:       "/usr/sbin/firecracker",
		UID:            intPtr(123),
		GID:            intPtr(900),
		NumaNode:       intPtr(0),
		Stderr:         os.Stderr,
		Stdin:          os.Stdin,
		Stdout:         os.Stdout,
		ChrootStrategy: sdk.NewNaiveChrootStrategy(vmConfig.KernelImagePath),
	}

	m, err := sdk.NewMachine(c, vmConfig)

	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(fcSocket)

	err = m.Start(ctx)
	if err != nil {
		log.Fatal(err)
	}

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

	go func() {
		if err := m.Wait(ctx); err != nil {
			fmt.Printf("VM exited with error: %v\n", err)
		} else {
			fmt.Println("VM has exited successfully.")
		}
		// Trigger additional logic here if needed when VM exits
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		s := <-sigCh
		log.Printf("got signal %v, attempting graceful shutdown", s)
		if err := m.StopVMM(); err != nil {
			log.Fatal(err)
		}
		//if err := m.Shutdown(ctx); err != nil {
		//	log.Fatal(err)
		//}
		cancel()
	}()

	<-ctx.Done()
}
