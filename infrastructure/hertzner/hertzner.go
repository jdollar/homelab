package hertzner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/pulumi/pulumi-hcloud/sdk/go/hcloud"
	"github.com/pulumi/pulumi-tls/sdk/v5/go/tls"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

const (
	hertznerSSHKeyName = "hertznerSshKey"
	resticSSHKeyName   = "hertznerResticSshKey"
)

var ErrFailedToCreateHertznerSSHKey = errors.New("failed to create hertzner ssh key")

const (
	configNamespaceHertzner = "hertzner"
	// configNamespaceInfrastructure = "infrastructure"
	configKeyStorageBoxes = "storage-boxes"
	configKeyApiToken     = "api-token"
	configKeyVmPassword   = "vm-password"
)

type storageBoxFeatures struct {
	ReachableExternally bool `json:"reachable-externally"`
	Samba               bool `json:"samba"`
	SSH                 bool `json:"ssh"`
	Webdav              bool `json:"webdav"`
	ZFS                 bool `json:"zfs"`
}

type snapshotPlan struct {
	Max       int `json:"max"`
	Minute    int `json:"minute"`
	Hour      int `json:"hour"`
	DayOfWeek int `json:"day-of-week"`
}

type storageBoxSubAccount struct {
	Password string `json:"password"`
}

type storageBoxConfig struct {
	DeleteProtection bool                            `json:"delete-protection"`
	BoxType          string                          `json:"type"`
	Location         string                          `json:"location"`
	Features         storageBoxFeatures              `json:"features"`
	SnapshotPlan     snapshotPlan                    `json:"snapshot-plan"`
	Subaccounts      map[string]storageBoxSubAccount `json:"subaccounts"`
}

func Run(ctx *pulumi.Context) error {
	hertznerConf := config.New(ctx, configNamespaceHertzner)

	apiToken := hertznerConf.RequireSecret(configKeyApiToken)
	vmPassword := hertznerConf.RequireSecret(configKeyVmPassword)

	provider, err := hcloud.NewProvider(ctx, "hertzner-provider", &hcloud.ProviderArgs{
		Token: apiToken,
	})
	if err != nil {
		return err
	}

	resourceOpts := []pulumi.ResourceOption{
		pulumi.Provider(provider),
	}

	// SSH Keys
	keys := map[string]*tls.PrivateKey{}
	names := []string{hertznerSSHKeyName, resticSSHKeyName}

	for _, keyName := range names {
		key, err := tls.NewPrivateKey(ctx, keyName, &tls.PrivateKeyArgs{
			Algorithm:  pulumi.String("ECDSA"),
			EcdsaCurve: pulumi.String("P384"),
		}, resourceOpts...)
		if err != nil {
			return errors.Join(ErrFailedToCreateHertznerSSHKey, err)
		}

		keys[keyName] = key
	}

	var storageBoxConfigs map[string]storageBoxConfig
	hertznerConf.RequireSecretObject(configKeyStorageBoxes, &storageBoxConfigs)

	keyInputs := pulumi.StringArray{}
	for _, key := range keys {
		keyInputs = append(
			keyInputs,
			key.PublicKeyOpenssh.ApplyT(func(publicKey string) string {
				return publicKey
			}).(pulumi.StringInput),
		)
	}

	for name, config := range storageBoxConfigs {
		box, err := hcloud.NewStorageBox(ctx, name, &hcloud.StorageBoxArgs{
			Name:           pulumi.String(name),
			StorageBoxType: pulumi.String(config.BoxType),
			Location:       pulumi.String(config.Location),
			Password:       vmPassword,
			SshKeys:        keyInputs,
			AccessSettings: &hcloud.StorageBoxAccessSettingsArgs{
				ReachableExternally: pulumi.Bool(config.Features.ReachableExternally),
				SambaEnabled:        pulumi.Bool(config.Features.Samba),
				SshEnabled:          pulumi.Bool(config.Features.SSH),
				WebdavEnabled:       pulumi.Bool(config.Features.Webdav),
				ZfsEnabled:          pulumi.Bool(config.Features.ZFS),
			},
			SnapshotPlan: &hcloud.StorageBoxSnapshotPlanArgs{
				MaxSnapshots: pulumi.Int(config.SnapshotPlan.Max),
				Minute:       pulumi.Int(config.SnapshotPlan.Minute),
				Hour:         pulumi.Int(config.SnapshotPlan.Hour),
				DayOfWeek:    pulumi.Int(config.SnapshotPlan.DayOfWeek),
			},
			DeleteProtection: pulumi.Bool(config.DeleteProtection),
		}, resourceOpts...)
		if err != nil {
			fmt.Println(err.Error())
			return err
		}

		boxID := box.ID().ApplyT(func(id string) int {
			val, _ := strconv.ParseInt(id, 10, 64)
			return int(val)
		})

		for accountName, accountConfig := range config.Subaccounts {
			_, err = hcloud.NewStorageBoxSubaccount(ctx, accountName, &hcloud.StorageBoxSubaccountArgs{
				StorageBoxId:  boxID.(pulumi.IntInput),
				HomeDirectory: pulumi.String(fmt.Sprintf("/user/%s/", accountName)),
				Password:      pulumi.String(accountConfig.Password),
				AccessSettings: &hcloud.StorageBoxSubaccountAccessSettingsArgs{
					ReachableExternally: pulumi.Bool(config.Features.ReachableExternally),
					SambaEnabled:        pulumi.Bool(config.Features.Samba),
					SshEnabled:          pulumi.Bool(config.Features.SSH),
					WebdavEnabled:       pulumi.Bool(config.Features.Webdav),
				},
			}, resourceOpts...)
			if err != nil {
				fmt.Println(err.Error())
				return err
			}
		}
	}

	// Store any generated ssh keys in dist to use for access
	for keyName, key := range keys {
		key.PublicKeyOpenssh.ApplyTWithContext(
			ctx.Context(),
			func(ctx context.Context, pubKey string) (string, error) {
				err := os.WriteFile(fmt.Sprintf("./dist/%s.pub", keyName), []byte(pubKey), 0o600)
				return pubKey, err
			},
		)

		key.PrivateKeyOpenssh.ApplyTWithContext(
			ctx.Context(),
			func(ctx context.Context, privKey string) (string, error) {
				err := os.WriteFile(fmt.Sprintf("./dist/%s", keyName), []byte(privKey), 0o600)
				return privKey, err
			},
		)
	}

	return nil
}
