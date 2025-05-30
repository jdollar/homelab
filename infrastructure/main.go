package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/muhlba91/pulumi-proxmoxve/sdk/v6/go/proxmoxve"
	"github.com/muhlba91/pulumi-proxmoxve/sdk/v6/go/proxmoxve/storage"
	"github.com/muhlba91/pulumi-proxmoxve/sdk/v6/go/proxmoxve/vm"
	"github.com/pulumi/pulumi-tls/sdk/v5/go/tls"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

const containerSSHKeyName = "containerSshKey"

const nodeNameAntebellum = "antebellum"

const (
	configNamespaceProxmoxve      = "proxmoxve"
	configNamespaceInfrastructure = "infrastructure"
	configKeyEndpoint             = "endpoint"
	configKeyInsecure             = "insecure"
	configKeyProxmoxvePassword    = "proxmoxve-password"
	configKeyProxmoxveUsername    = "proxmoxve-username"
	configKeyVMs                  = "vms"
	configKeyVMUserPassword       = "vm-user-password"
	configKeyVMUserName           = "vm-user-name"
	configKeyGatewayIP            = "gateway-ip"
	configKeyTemplates            = "templates"
)

const (
	fileDebian12ISOName        = "debian12iso"
	fileDebian12ISOUrl         = "https://cdimage.debian.org/debian-cd/current/amd64/iso-cd/debian-12.1.0-amd64-netinst.iso"
	fileDebian12CTTemplateName = "debian12cloudinit"
	fileDebian12CTTemplateUrl  = "http://download.proxmox.com/images/system/debian-12-standard_12.0-1_amd64.tar.zst"
)

const dataStoreIDNas = "nas-proxmox"

const (
	contentTypeISO    = "iso"
	contentTypeVZTmpl = "vztmpl"
)

var (
	ErrFailedToCreateProvider        = errors.New("failed to create provider")
	ErrFailedToCreateContainerSSHKey = errors.New("failed to create container ssh key")
	ErrFailedToCreateIsoFile         = errors.New("failed to create iso file")
	ErrFailedToCreateContainerFile   = errors.New("failed to create container file")
	ErrFailedToGetPublicSSHKey       = errors.New("failed to get public ssh key")
	ErrFailedToGetPrivateSSHKey      = errors.New("failed to get private ssh key")
	ErrFailedToCreateVM              = errors.New("failed to create vm")
	ErrInvalidTemplateOnVM           = errors.New("invalid template on vm config")
)

type Template struct {
	VMID                int    `json:"vm-id"`
	NodeName            string `json:"node-name"`
	DataStoreID         string `json:"datastore-id"`
	OperatingSystemType string `json:"operating-system-type"`
}

type VM struct {
	CpuCores        int    `json:"cpu-cores"`
	Template        string `json:"template"`
	IP              string `json:"ip"`
	NodeName        string `json:"node-name"`
	MACAddress      string `json:"mac-address"`
	DiskSize        int    `json:"disk-size"`
	DedicatedMemory int    `json:"dedicated-memory"`
	DiskDataStoreID string `json:"disk-datastore-id"`
	Machine         string `json:"machine"`
	VGAType         string `json:"vga-type"`
}

func run(ctx *pulumi.Context) error {
	conf := config.New(ctx, "")
	infraConf := config.New(ctx, configNamespaceInfrastructure)
	proxmoxConf := config.New(ctx, configNamespaceProxmoxve)

	username := infraConf.Require(configKeyProxmoxveUsername)
	password := infraConf.Require(configKeyProxmoxvePassword)
	vmUserName := infraConf.Require(configKeyVMUserName)
	vmUserPassword := infraConf.RequireSecret(configKeyVMUserPassword)
	gatewayIP := infraConf.Require(configKeyGatewayIP)

	proxmoxEndpoint := proxmoxConf.Require(configKeyEndpoint)
	proxmoxInsecure := proxmoxConf.RequireBool(configKeyInsecure)

	provider, err := proxmoxve.NewProvider(ctx, "proxmoxve", &proxmoxve.ProviderArgs{
		Endpoint: pulumi.String(proxmoxEndpoint),
		Insecure: pulumi.BoolPtr(proxmoxInsecure),
		Username: pulumi.String(username),
		Password: pulumi.String(password),
	})
	if err != nil {
		return ErrFailedToCreateProvider
	}

	resourceOpts := []pulumi.ResourceOption{
		pulumi.Provider(provider),
	}

	// SSH Key
	containerSSHKey, err := tls.NewPrivateKey(ctx, containerSSHKeyName, &tls.PrivateKeyArgs{
		Algorithm:  pulumi.String("ECDSA"),
		EcdsaCurve: pulumi.String("P384"),
	}, resourceOpts...)
	if err != nil {
		return ErrFailedToCreateContainerSSHKey
	}

	containerSSHKey.PublicKeyOpenssh.ApplyTWithContext(
		ctx.Context(),
		func(ctx context.Context, pubKey string) (string, error) {
			err := os.WriteFile("./dist/containerSshKey.pub", []byte(pubKey), 0o600)
			return pubKey, err
		},
	)

	containerSSHKey.PrivateKeyOpenssh.ApplyTWithContext(
		ctx.Context(),
		func(ctx context.Context, privKey string) (string, error) {
			err := os.WriteFile("./dist/containerSshKey", []byte(privKey), 0o600)
			return privKey, err
		},
	)

	// Files
	_, err = storage.NewFile(
		ctx,
		fileDebian12ISOName,
		&storage.FileArgs{
			NodeName:    pulumi.String(nodeNameAntebellum),
			DatastoreId: pulumi.String(dataStoreIDNas),
			ContentType: pulumi.String(contentTypeISO),
			SourceFile: &storage.FileSourceFileArgs{
				Path: pulumi.String(fileDebian12ISOUrl),
			},
		},
		resourceOpts...,
	)
	if err != nil {
		return ErrFailedToCreateIsoFile
	}

	_, err = storage.NewFile(
		ctx,
		fileDebian12CTTemplateName,
		&storage.FileArgs{
			NodeName:    pulumi.String(nodeNameAntebellum),
			DatastoreId: pulumi.String(dataStoreIDNas),
			ContentType: pulumi.String(contentTypeVZTmpl),
			SourceFile: &storage.FileSourceFileArgs{
				Path: pulumi.String(fileDebian12CTTemplateUrl),
			},
		},
		resourceOpts...,
	)
	if err != nil {
		return ErrFailedToCreateContainerFile
	}

	// VMs
	var templateConfigs map[string]Template
	conf.RequireObject(configKeyTemplates, &templateConfigs)

	var vmConfigs map[string]VM
	conf.RequireObject(configKeyVMs, &vmConfigs)

	for vmName, vmConfig := range vmConfigs {
		templateConfig, ok := templateConfigs[vmConfig.Template]
		if !ok {
			return fmt.Errorf("%w: %s", ErrInvalidTemplateOnVM, vmName)
		}

		vmArgs := vm.VirtualMachineArgs{
			BootOrders: pulumi.StringArray{
				pulumi.String("virtio0"),
				pulumi.String("ide2"),
			},
			Clone: vm.VirtualMachineCloneArgs{
				VmId:        pulumi.Int(templateConfig.VMID),
				DatastoreId: pulumi.String(templateConfig.DataStoreID),
				NodeName:    pulumi.String(templateConfig.NodeName),
			},
			Cpu: vm.VirtualMachineCpuArgs{
				Cores:      pulumi.IntPtr(vmConfig.CpuCores),
				Flags:      pulumi.StringArray{},
				Type:       pulumi.String("host"),
				Numa:       pulumi.Bool(true),
				Hotplugged: pulumi.Int(0),
			},
			Cdrom: vm.VirtualMachineCdromArgs{
				Enabled:   pulumi.Bool(false),
				Interface: pulumi.String("ide2"),
				FileId:    pulumi.String("none"),
			},
			Disks: vm.VirtualMachineDiskArray{
				vm.VirtualMachineDiskArgs{
					Cache:       pulumi.String("none"),
					Size:        pulumi.Int(vmConfig.DiskSize),
					DatastoreId: pulumi.String(vmConfig.DiskDataStoreID),
					Interface:   pulumi.String("virtio0"),
					FileFormat:  pulumi.String("raw"),
				},
			},
			Initialization: vm.VirtualMachineInitializationArgs{
				Dns: vm.VirtualMachineInitializationDnsArgs{
					Server: pulumi.String(gatewayIP),
				},
				Interface: pulumi.String("ide2"),
				IpConfigs: vm.VirtualMachineInitializationIpConfigArray{
					vm.VirtualMachineInitializationIpConfigArgs{
						Ipv4: vm.VirtualMachineInitializationIpConfigIpv4Args{
							Address: pulumi.String(vmConfig.IP),
							Gateway: pulumi.String(gatewayIP),
						},
					},
				},
				Type: pulumi.String("nocloud"),
				UserAccount: vm.VirtualMachineInitializationUserAccountArgs{
					Keys: pulumi.StringArray{
						containerSSHKey.PublicKeyOpenssh.ApplyT(func(publicKey string) string {
							return publicKey
						}).(pulumi.StringInput),
					},
					Password: vmUserPassword,
					Username: pulumi.String(vmUserName),
				},
			},
			Machine: pulumi.String(vmConfig.Machine),
			Memory: vm.VirtualMachineMemoryArgs{
				Dedicated: pulumi.IntPtr(vmConfig.DedicatedMemory),
			},
			Name: pulumi.String(vmName),
			NetworkDevices: vm.VirtualMachineNetworkDeviceArray{
				vm.VirtualMachineNetworkDeviceArgs{
					MacAddress: pulumi.String(vmConfig.MACAddress),
				},
			},
			NodeName: pulumi.String(vmConfig.NodeName),
			OperatingSystem: vm.VirtualMachineOperatingSystemArgs{
				Type: pulumi.String(templateConfig.OperatingSystemType),
			},
			Protection:    pulumi.Bool(true),
			ScsiHardware:  pulumi.String("virtio-scsi-single"),
			SerialDevices: vm.VirtualMachineSerialDeviceArray{},
			Started:       pulumi.Bool(true),
		}

		if vmConfig.VGAType != "" {
			vmArgs.Vga = vm.VirtualMachineVgaArgs{
				Type: pulumi.String(vmConfig.VGAType),
			}
		}

		vmResourceOpts := append(
			resourceOpts,
			pulumi.IgnoreChanges([]string{
				"initialization",
				"disks[0].speed",
				"clone",
			}),
		)

		_, err := vm.NewVirtualMachine(ctx, vmName, &vmArgs, vmResourceOpts...)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrFailedToCreateVM, vmName)
		}
	}

	return nil
}

func main() {
	err := pulumi.RunErr(run)
	if err != nil {
		log.Printf("Error: %w", err)
		os.Exit(1)
	}
}
