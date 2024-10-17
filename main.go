package main

import (
	"context"
	"encoding/json"
	"fmt"
	sdk "github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"github.com/google/uuid"
	"net"
	"os"
	"os/signal"
	ssh_gen "runners-service/internal"
	"syscall"

	log "github.com/sirupsen/logrus"
)

type PublicKey struct {
	OpensshKey string `json:"openssh-key"`
}

type Metadata struct {
	PublicKeys map[string]PublicKey `json:"public-keys"`
}

type Latest struct {
	Metadata Metadata `json:"meta-data"`
}

type Data struct {
	Latest Latest `json:"latest"`
}

func boolPtr(b bool) *bool {
	return &b
}

func intPtr(i int) *int {
	return &i
}

func stringPtr(s string) *string {
	return &s
}

func createNewConfig() sdk.Config {
	dir, _ := os.Getwd()
	fmt.Println(dir)
	kernelImage := "/home/brian/custom-alpine/vmlinux-6.1.102"

	var vcpuCount int64 = 1
	var memSizeMib int64 = 1024
	smt := false

	driveID := "root"
	pathOnHost := "/home/brian/custom-alpine/alpine.ext4"

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

	vmConfig := createNewConfig()
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

	machineId := uuid.New().String()

	nullio, err := os.Open("/dev/null")
	vmConfig.JailerCfg = &sdk.JailerConfig{
		ID:             machineId,
		JailerBinary:   "jailer",
		ExecFile:       "/usr/sbin/firecracker",
		UID:            intPtr(123),
		GID:            intPtr(900),
		NumaNode:       intPtr(0),
		Stderr:         nullio,
		Stdin:          nullio,
		Stdout:         nullio,
		ChrootStrategy: sdk.NewNaiveChrootStrategy(vmConfig.KernelImagePath),
		CgroupVersion:  "2",
		ChrootBaseDir:  "/srv/jailer",
	}

	// Ensure the KernelImagePath and each configured drive path files are owned by the jailer
	if vmConfig.JailerCfg != nil {
		changeFileOwner(vmConfig.KernelImagePath, *vmConfig.JailerCfg.UID, *vmConfig.JailerCfg.GID)

		for _, drive := range vmConfig.Drives {
			changeFileOwner(*drive.PathOnHost, *vmConfig.JailerCfg.UID, *vmConfig.JailerCfg.GID)
		}
	}

	vmConfig.VMID = machineId
	vmConfig.NetNS = fmt.Sprintf("/var/run/netns/%s", machineId)

	logger := log.New()

	log.SetLevel(log.DebugLevel)
	logger.SetLevel(log.DebugLevel)

	machineOpts := []sdk.Opt{
		sdk.WithLogger(log.NewEntry(logger)),
	}

	m, err := sdk.NewMachine(c, vmConfig, machineOpts...)
	if err != nil {
		log.Fatal(err)
	}

	err = m.Start(ctx)
	if err != nil {
		log.Fatal(err)
	}

	idrsaPath := fmt.Sprintf("%s/firecracker/%s/root/id_rsa", m.Cfg.JailerCfg.ChrootBaseDir, vmConfig.VMID)
	opensshKey, _ := ssh_gen.GenerateMachineKeys(idrsaPath)

	metaDataIP := m.Cfg.MmdsAddress.String()
	log.Printf("Metadata IP: %v\n", metaDataIP)

	err = generateMetaData(c, m, opensshKey)
	if err != nil {
		log.Fatal(err)
	}

	vmIP := m.Cfg.NetworkInterfaces[0].StaticConfiguration.IPConfiguration.IPAddr.IP.String()
	vmMAC := m.Cfg.NetworkInterfaces[0].StaticConfiguration.MacAddress
	log.Printf("MAC of VM: %v\n", vmMAC)
	log.Printf("IP of VM: %v\n", vmIP)

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

func generateMetaData(ctx context.Context, m *sdk.Machine, key string) error {
	data := Data{
		Latest: Latest{
			Metadata: Metadata{
				PublicKeys: map[string]PublicKey{
					"0": {
						OpensshKey: key,
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	var metadata interface{}
	err = json.Unmarshal(jsonData, &metadata)
	if err != nil {
		return err
	}

	m.SetMetadata(ctx, metadata)

	return nil
}

func changeFileOwner(filename string, uid int, gid int) error {
	return os.Chown(filename, uid, gid)
}
