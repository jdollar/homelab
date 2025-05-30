# Configuration as Code

## Overview

This section of the repository houses an ansible playbooks that
runs through and configures all the VMs, VPCs, Raspberry Pis or
whatever else I have on my network.

The inventory is based on IP addresses so the expectation is that
the individual items will have a pre-determined IP set at either the
router or ISP level.

### Roles

There are a number of ansible roles that are associated to one or more
machine in the inventory. A machine can have more than one role. A role in my
situation handles setting up a specific feature on a given machine. A brief overview
of what each role handles is below:

- `keepalived` handles installing (https://www.keepalived.org/)[https://www.keepalived.org/] to facilitate various failover scenarios
- `dns` configures a server to have a keepalived setup to facilitate dns lookups
- `k3smaster` setups up a non-HA k3s node
- `k3sagent` setups up a k3s agent node which will use the token from the k3smaster machine
- `manifestrunner` houses and places "manifest" files to install using k3s (https://docs.k3s.io/installation/packaged-components)[https://docs.k3s.io/installation/packaged-components]
  - This role primarily houses some manifests that need to be run prior to argocd being spun up in a cluster (including argocd itself)
- `outerproxy` setups up a vpc with a reverseproxy facilitating tunneling traffic to the local network from the internet
- `ssh` configures the ssh server out there to listen to a non-standard port
- `minecraftproxy` setups up a reverse proxy specific to proxy minecraft tcp traffic and VOIP functionalities with mods

The actual task files themselves are complete enough to set things up, but they do probably need to be
cleaned up at some point to ensure commands aren't run excessively and that they are
structured in a more understandable way.

## Manual Steps

This section will detail information on any manual steps needed
to be performed on any of the machines in the inventory that
haven't yet been automated.

### Dell Optiplex Machines

#### Grub Updates

Had numerous issues with the nics in dell optiplex 7070 SFF going down
when it was receiving decent traffic. Additionally there were issues
in those boxes around the nvme going into a read-only state. as well.
To fix both I made sure the grub command line args were set to the following in `/etc/default/grub`.

```bash
GRUB_CMDLINE_LINUX_DEFAULT="quiet intel_iommu=on nvme_core.default_ps_max_latency_us=0"
```

- `intel_iommu=on` - helps prevent intel wifi hang
- `nvme_core.default_ps_max_latency_us=0` - helps prevent nvme read-only state

Then we need to update grub and reboot

```bash
sudo update-grub
sudo reboot now
```

#### Network Updates

Update network interfaces to disable offloading for optiplex boxes (intel wifi)
This prevents the nics from going into a hardware hang state

```
auto eth0
iface eth0 inet static
  offload-gso off
  offload-gro off
  offload-tso off
  offload-rx off
  offload-tx off
  offload-rxvlan off
  offload-txvlan off
  offload-sg off
  offload-ufo off
  offload-lro off
```

## Pre-requisites

The only pre-requisite needed here is that the Infrastructure as Code
solution has been invoked. This ansible playbook does handle setting up
the individual resources spun up in that process and connections will
utilize the ssh key present there if configured.

## Running

Everything is contained within a single playbook and you can limit it based
on the host you wish to run it against. The entire playbook can be invoked like so:

```shell
ansible-playbook playbook.yml \
    -i inventory.yaml \
    --ask-become-pass \
    --ask-vault-pass \
    --key-file ../infrastructure/dist/containerSshKey
```

## Updating

To facilitate running update on the various package managers out there
I created a second playbook which can be invoked via:

```shell
ansible-playbook update-playbook.yml \
    -i inventory.yaml \
    --ask-become-pass \
    --key-file ../infrastructure/dist/containerSshKey
```
