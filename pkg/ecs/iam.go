package ecs

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/devsy-org/devsy-provider-ecs/pkg/options"
	"github.com/devsy-org/devsy/pkg/log"
)

var (
	devsyRoleName   = "devsy-ecs-role"
	devsyPolicyName = "devsy-ecs-policy"
)

const iamPolicyDocument = `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Action": [
                "ecs:ExecuteCommand",
                "ssmmessages:CreateControlChannel",
                "ssmmessages:CreateDataChannel",
                "ssmmessages:OpenControlChannel",
                "ssmmessages:OpenDataChannel",
                "logs:CreateLogStream",
                "logs:PutLogEvents"
            ],
            "Effect": "Allow",
            "Resource": "*"
        }
    ]
}`

const iamAssumeRolePolicyDocument = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "ecs-tasks.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}`

func (p *EcsProvider) createIamRole(ctx context.Context) (string, error) {
	iamClient := iam.NewFromConfig(p.AwsConfig)

	// reuse the existing role if present
	if arn, found, err := p.existingRoleARN(ctx, iamClient); err != nil {
		return "", err
	} else if found {
		return arn, nil
	}

	// create policy
	log.Infof("Create iam policy %s...", devsyPolicyName)
	policyOutput, err := iamClient.CreatePolicy(ctx, &iam.CreatePolicyInput{
		PolicyName:     &devsyPolicyName,
		PolicyDocument: options.Ptr(iamPolicyDocument),
	})
	if err != nil {
		return "", fmt.Errorf("create policy: %w", err)
	}

	// create role
	log.Infof("Create iam role %s...", devsyRoleName)
	roleOutput, err := iamClient.CreateRole(ctx, &iam.CreateRoleInput{
		RoleName:                 &devsyRoleName,
		AssumeRolePolicyDocument: options.Ptr(iamAssumeRolePolicyDocument),
	})
	if err != nil {
		p.deletePolicy(ctx, iamClient, policyOutput.Policy.Arn)
		return "", fmt.Errorf("create iam role: %w", err)
	}

	// attach policy
	log.Infof("Attach iam policy %s to role %s...", devsyPolicyName, devsyRoleName)
	_, err = iamClient.AttachRolePolicy(ctx, &iam.AttachRolePolicyInput{
		PolicyArn: policyOutput.Policy.Arn,
		RoleName:  &devsyRoleName,
	})
	if err != nil {
		p.deletePolicy(ctx, iamClient, policyOutput.Policy.Arn)
		_, _ = iamClient.DeleteRole(ctx, &iam.DeleteRoleInput{RoleName: &devsyRoleName})
		return "", fmt.Errorf("attach iam policy to role: %w", err)
	}

	return *roleOutput.Role.Arn, nil
}

// existingRoleARN returns the ARN of the shared role if it already exists.
func (p *EcsProvider) existingRoleARN(
	ctx context.Context,
	iamClient *iam.Client,
) (string, bool, error) {
	role, err := iamClient.GetRole(ctx, &iam.GetRoleInput{
		RoleName: &devsyRoleName,
	})
	if err != nil {
		var re *awshttp.ResponseError
		if !errors.As(err, &re) || re.HTTPStatusCode() != http.StatusNotFound {
			return "", false, err
		}

		return "", false, nil
	}

	return *role.Role.Arn, true, nil
}

func (p *EcsProvider) deletePolicy(ctx context.Context, iamClient *iam.Client, policyArn *string) {
	_, _ = iamClient.DeletePolicy(ctx, &iam.DeletePolicyInput{PolicyArn: policyArn})
}
