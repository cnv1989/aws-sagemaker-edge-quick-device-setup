package aws

import (
	"aws-sagemaker-edge-quick-device-setup/cli"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
)

type IamClient interface {
	CreateRole(ctx context.Context, params *iam.CreateRoleInput, optFns ...func(*iam.Options)) (*iam.CreateRoleOutput, error)
	GetRole(ctx context.Context, params *iam.GetRoleInput, optFns ...func(*iam.Options)) (*iam.GetRoleOutput, error)
	ListAttachedRolePolicies(ctx context.Context, params *iam.ListAttachedRolePoliciesInput, optFns ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error)
	AttachRolePolicy(ctx context.Context, params *iam.AttachRolePolicyInput, optFns ...func(*iam.Options)) (*iam.AttachRolePolicyOutput, error)
	GetPolicy(ctx context.Context, params *iam.GetPolicyInput, optFns ...func(*iam.Options)) (*iam.GetPolicyOutput, error)
	CreatePolicy(ctx context.Context, params *iam.CreatePolicyInput, optFns ...func(*iam.Options)) (*iam.CreatePolicyOutput, error)
}

func CreateDeviceFleetRole(client IamClient, fleetName *string, roleName *string) *types.Role {
	assumeRolePolicyDocument := `{
		"Version": "2012-10-17",
		"Statement": [
			{
			  "Effect": "Allow",
			  "Principal": {"Service": "credentials.iot.amazonaws.com"},
			  "Action": ["sts:AssumeRole"]
			},
			{
			  "Effect": "Allow",
			  "Principal": {"Service": "sagemaker.amazonaws.com"},
			  "Action": ["sts:AssumeRole"]
			}
		]
	}`

	result, err := client.CreateRole(context.TODO(), &iam.CreateRoleInput{
		AssumeRolePolicyDocument: &assumeRolePolicyDocument,
		RoleName:                 roleName,
	})

	if err != nil {
		log.Fatalf("Failed to create role with role name %s. Encountered Error %s\n", *roleName, err)
	}

	return result.Role
}

func GetDeviceFleetRole(client IamClient, fleetName *string, roleName *string) *types.Role {
	result, err := client.GetRole(context.TODO(), &iam.GetRoleInput{
		RoleName: roleName,
	})

	if err != nil {
		var nse *types.NoSuchEntityException
		if errors.As(err, &nse) {
			log.Println("Role doesn't exist.")
			return nil
		}
		log.Fatalf("Failed to get role with role name %s. Encountered error %s\n", *roleName, err)
	}

	return result.Role
}

func CheckIfPolicyIsAlreadyAttachedToTheRole(client IamClient, roleName *string, policyName *string) *types.AttachedPolicy {
	maxItems := int32(100)
	var marker *string

	for {
		ret, err := client.ListAttachedRolePolicies(context.TODO(), &iam.ListAttachedRolePoliciesInput{
			RoleName: roleName,
			MaxItems: &maxItems,
			Marker:   marker,
		})

		if err != nil {
			log.Fatalf("Failed to list attached role policies for %s. Encountered Error %s\n", *roleName, err)
		}

		for _, policy := range ret.AttachedPolicies {
			if *policy.PolicyName == *policyName {
				return &policy
			}
		}

		if ret.IsTruncated {
			marker = ret.Marker
		} else {
			break
		}
	}

	return nil
}

func AttachAmazonSageMakerEdgeDeviceFleetPolicy(client IamClient, role *types.Role, policyArn *string) {
	_, err := client.AttachRolePolicy(context.TODO(), &iam.AttachRolePolicyInput{
		PolicyArn: policyArn,
		RoleName:  role.RoleName,
	})

	if err != nil {
		log.Fatalf("Failed to attach policy %s to role name %s. Encountered error %s\n", *policyArn, *role.RoleName, err)
	}
}

type Principal struct {
	Service string `json:",omitempty"`
}

type StatementEntry struct {
	Sid       string `json:",omitempty"`
	Effect    string
	Action    []string
	Resource  []string
	Condition map[string]interface{} `json:",omitempty"`
	Principal *Principal             `json:",omitempty"`
}

type PolicyDocument struct {
	Version   string
	Statement []StatementEntry
}

func CreateDeviceFleetBucketPolicy(client IamClient, cliArgs *cli.CliArgs) *types.Policy {
	policyDocument := &PolicyDocument{
		Version: "2012-10-17",
		Statement: []StatementEntry{
			{
				Sid:    "DeviceS3Access",
				Effect: "Allow",
				Action: []string{
					"s3:PutObject",
					"s3:GetBucketLocation",
				},
				Resource: []string{
					fmt.Sprintf("arn:aws:s3:::%s/*", cliArgs.DeviceFleetBucket),
					fmt.Sprintf("arn:aws:s3:::%s", cliArgs.DeviceFleetBucket),
				},
			},
		},
	}
	policy, _ := json.MarshalIndent(policyDocument, "", " ")
	policyDoc := string(policy)

	policyDescription := fmt.Sprintf("SageMaker device fleet bucket policy for %s", cliArgs.DeviceFleet)
	policyPath := "/"
	policyName := fmt.Sprintf("%s-%s-policy", strings.ToLower(cliArgs.DeviceFleet), strings.ToLower(cliArgs.DeviceFleetBucket))
	policyArn := fmt.Sprintf("arn:aws:iam::%s:policy/%s", cliArgs.Account, policyName)

	getPolicyOutput, err := client.GetPolicy(context.TODO(), &iam.GetPolicyInput{
		PolicyArn: &policyArn,
	})

	if err != nil {
		var nse *types.NoSuchEntityException
		if errors.As(err, &nse) {
			ret, err := client.CreatePolicy(context.TODO(), &iam.CreatePolicyInput{
				Description:    &policyDescription,
				Path:           &policyPath,
				PolicyDocument: &policyDoc,
				PolicyName:     &policyName,
			})

			if err != nil {
				log.Fatalf("Failed to create policy with policy name %s. Encountered error %s\n", policyName, err)
			}

			return ret.Policy
		}

		log.Fatalf("Failed to get policy with name %s. Encountered error %s\n", policyName, err)
	}

	return getPolicyOutput.Policy
}

func CreateDeviceFleetPolicy(client IamClient, cliArgs *cli.CliArgs) *types.Policy {
	var condition map[string]interface{}
	conditionByt := []byte(` {
		"StringEqualsIfExists": {
			"iam:PassedToService": [
				"iot.amazonaws.com",
				"credentials.iot.amazonaws.com"
			]
		}
	}`)

	if err := json.Unmarshal(conditionByt, &condition); err != nil {
		log.Fatal("Invaild json doc. Encountered err ", err)
	}

	policyDocument := &PolicyDocument{
		Version: "2012-10-17",
		Statement: []StatementEntry{
			{
				Sid:    "SageMakerEdgeApis",
				Effect: "Allow",
				Action: []string{
					"sagemaker:SendHeartbeat",
					"sagemaker:GetDeviceRegistration",
				},
				Resource: []string{
					fmt.Sprintf("arn:aws:sagemaker:%s:%s:device-fleet/%s/device/*", cliArgs.Region, cliArgs.Account, strings.ToLower(cliArgs.DeviceFleet)),
					fmt.Sprintf("arn:aws:sagemaker:%s:%s:device-fleet/%s", cliArgs.Region, cliArgs.Account, strings.ToLower(cliArgs.DeviceFleet)),
				},
			},
			{
				Sid:    "CreateIOTRoleAlias",
				Effect: "Allow",
				Action: []string{
					"iot:CreateRoleAlias",
					"iot:DescribeRoleAlias",
					"iot:UpdateRoleAlias",
					"iot:ListTagsForResource",
					"iot:TagResource",
				},
				Resource: []string{
					fmt.Sprintf("arn:aws:iot:%s:%s:rolealias/SageMakerEdge-%s", cliArgs.Region, cliArgs.Account, cliArgs.DeviceFleet),
				},
			},
			{
				Sid:    "CreateIoTRoleAliasIamPermissionsGetRole",
				Effect: "Allow",
				Action: []string{
					"iam:GetRole",
				},
				Resource: []string{
					fmt.Sprintf("arn:aws:iam::%s:role/%s", cliArgs.Account, cliArgs.DeviceFleetRole),
				},
			},
			{
				Sid:    "CreateIoTRoleAliasIamPermissionsPassRole",
				Effect: "Allow",
				Action: []string{
					"iam:PassRole",
				},
				Resource: []string{
					fmt.Sprintf("arn:aws:iam::%s:role/%s", cliArgs.Account, cliArgs.DeviceFleetRole),
				},
				Condition: condition,
			},
		},
	}
	policy, _ := json.MarshalIndent(policyDocument, "", " ")
	policyDoc := string(policy)

	policyDescription := fmt.Sprintf("SageMaker device fleet policy for %s", cliArgs.DeviceFleet)
	policyPath := "/"
	policyName := fmt.Sprintf("%s-policy", strings.ToLower(cliArgs.DeviceFleet))
	policyArn := fmt.Sprintf("arn:aws:iam::%s:policy/%s", cliArgs.Account, policyName)

	getPolicyOutput, err := client.GetPolicy(context.TODO(), &iam.GetPolicyInput{
		PolicyArn: &policyArn,
	})

	if err != nil {
		var nse *types.NoSuchEntityException
		if errors.As(err, &nse) {
			ret, err := client.CreatePolicy(context.TODO(), &iam.CreatePolicyInput{
				Description:    &policyDescription,
				Path:           &policyPath,
				PolicyDocument: &policyDoc,
				PolicyName:     &policyName,
			})

			if err != nil {
				log.Fatalf("Failed to create policy with name %s. Encountered error %s\n", policyName, err)
			}

			return ret.Policy
		}

		log.Fatalf("Failed to get policy with name %s. Encountered error %s\n", policyName, err)
	} else {
		log.Println("Policy already exists in the account!")
	}

	return getPolicyOutput.Policy
}

func CreateDeviceFleetRoleIfNotExists(client IamClient, fleetName *string, roleName *string, fleetPolicy *types.Policy, bucketPolicy *types.Policy) *types.Role {
	role := GetDeviceFleetRole(client, fleetName, roleName)
	if role == nil {
		role = CreateDeviceFleetRole(client, fleetName, roleName)
	}
	attachedFleetPolicy := CheckIfPolicyIsAlreadyAttachedToTheRole(client, role.RoleName, fleetPolicy.PolicyName)

	if attachedFleetPolicy == nil {
		log.Println("Attaching device fleet policy")
		AttachAmazonSageMakerEdgeDeviceFleetPolicy(client, role, fleetPolicy.Arn)
	}

	attachedBucketPolicy := CheckIfPolicyIsAlreadyAttachedToTheRole(client, role.RoleName, bucketPolicy.PolicyName)

	if attachedBucketPolicy == nil {
		log.Println("Attaching device fleet bucket policy")
		AttachAmazonSageMakerEdgeDeviceFleetPolicy(client, role, bucketPolicy.Arn)
	}
	return role
}
