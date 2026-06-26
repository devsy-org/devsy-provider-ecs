# ECS Provider for Devsy

## Getting started

The provider is available for auto-installation using

```sh
devsy provider add ecs
devsy provider use ecs
```

Follow the on-screen instructions to complete the setup.

Needed variables will be:

- **CLUSTER_ID**: ECS Cluster ID either as ARN or ID
- **SUBNET_ID**: ECS Subnet ID either as ARN or ID to run the tasks in. This can either be a private subnet with a NAT Gateway or a Public Subnet. Depending on the type of the subnet you will need to set ASSIGN_PUBLIC_IP accordingly

The provider will inherit the login information from `aws cli` or you can
specify in your environment the `AWS_ACCESS_KEY_ID=`
and `AWS_SECRET_ACCESS_KEY=` variables.

### Creating your first Devsy env with ECS

After the initial setup, just use:

```sh
devsy up .
```

You'll need to wait for the task and environment setup.

## Local Development

To build and test the provider locally, use [task](https://taskfile.dev/) `task build:provider:dev`. The provider file is created in `./dist/provider.yaml`.
