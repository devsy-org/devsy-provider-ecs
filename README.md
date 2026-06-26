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

### Customize the Task

This provider has the following options:

| NAME                 | REQUIRED | DESCRIPTION                                                                                              | DEFAULT  |
| -------------------- | -------- | ------------------------------------------------------------------------------------------------------- | -------- |
| CLUSTER_ID           | true     | ECS Cluster ID either as ARN or ID.                                                                     |          |
| SUBNET_ID            | true     | ECS Subnet ID (ARN or ID) to run the tasks in. Set ASSIGN_PUBLIC_IP according to the subnet type.       |          |
| AWS_PROFILE          | false    | The aws profile name to use.                                                                            | default  |
| TASK_ROLE_ARN        | false    | ECS Task Role ARN for the task definition. If unset, the provider tries to create a new role.           |          |
| EXECUTION_ROLE_ARN   | false    | ECS Execution Role ARN for the task definition. If unset, the provider tries to create a new role.      |          |
| CLUSTER_ARCHITECTURE | false    | The cpu architecture of the cluster. Either amd64 or arm64.                                             | amd64    |
| TASK_CPU             | false    | ECS Task cpu as a string. With Fargate, ensure the TASK_MEMORY combination is supported. E.g. '.5 vcpu'. | 2 vcpu   |
| TASK_MEMORY          | false    | ECS Task memory as a string. With Fargate, ensure the TASK_CPU combination is supported. E.g. '1 gb'.    | 4 gb     |
| LAUNCH_TYPE          | false    | ECS Task Launch Type. One of FARGATE, EC2, or EXTERNAL.                                                  | FARGATE  |
| ASSIGN_PUBLIC_IP     | false    | Whether the task gets a public ip. ENABLED for public subnets, DISABLED for private subnets.            | ENABLED  |
| SECURITY_GROUP_ID    | false    | ECS Security Group ID to attach to the network settings of the ECS task.                                |          |

Options can either be set in `env` or on the command line, for example:

```sh
devsy provider set-options -o TASK_CPU="1 vcpu" -o TASK_MEMORY="2 gb"
```

## Local Development

To build and test the provider locally, use [task](https://taskfile.dev/) `task build:provider:dev`. The provider file is created in `./dist/provider.yaml`.
