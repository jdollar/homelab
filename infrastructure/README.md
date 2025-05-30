# Infrastructure as Code

## Overview

The infrastructure here is managed by Pulumi using the go SDK for it.
This pulumi stack sets up a number of resources on a proxmox host.

View pulumi config files in the infrasturcture directory for possible values you
may want to modify `pulumi.prod.yaml` or `pulumi.yaml`

```bash
go mod tidy
pulumi up
```

## Outputs

After the infrastructure has been spun up, there will be a new `/dist` directory.
There will be a private and public key in that directory that can be used
to ssh into the created VMs.
