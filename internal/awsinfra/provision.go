package awsinfra

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type Infra struct {
	EC2    *ec2.Client
	SSM    *ssm.Client
	Region string
}

func New(ctx context.Context, region string) (*Infra, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))

	if err != nil {
		return nil, err
	}
	return &Infra{
		EC2:    ec2.NewFromConfig(cfg),
		SSM:    ssm.NewFromConfig(cfg),
		Region: region,
	}, nil
}

func tags(rt types.ResourceType, name string) []types.TagSpecification {
	return []types.TagSpecification{
		{
			ResourceType: rt,
			Tags: []types.Tag{
				{Key: aws.String("zeroku"), Value: aws.String("true")},
				{Key: aws.String("name"), Value: aws.String(name)},
			},
		},
	}
}

func myPublicIP(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "https://checkip.amazonaws.com", nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func (i *Infra) createKeyPair(ctx context.Context, name, pemPath string) error {
	out, err := i.EC2.CreateKeyPair(ctx, &ec2.CreateKeyPairInput{
		KeyName:           aws.String(name),
		KeyType:           types.KeyTypeEd25519,
		TagSpecifications: tags(types.ResourceTypeKeyPair, name),
	})
	if err != nil {
		return err
	}
	return os.WriteFile(pemPath, []byte(aws.ToString(out.KeyMaterial)), 0o400)
}

func (i *Infra) createSecurityGroup(ctx context.Context, name, vpcID, myCIDR string) (string, error) {
	out, err := i.EC2.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:         aws.String(name),
		Description:       aws.String("zeroku"),
		VpcId:             aws.String(vpcID),
		TagSpecifications: tags(types.ResourceTypeSecurityGroup, name),
	})
	if err != nil {
		return "", err
	}
	sgID := aws.ToString(out.GroupId)

	_, err = i.EC2.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: aws.String(sgID),
		IpPermissions: []types.IpPermission{
			tcpIngress(80, "0.0.0.0/0", "http"),
			tcpIngress(443, "0.0.0.0/0", "https"),
			tcpIngress(22, myCIDR, "ssh"),
		},
	})
	if err != nil {
		return "", err
	}

	return sgID, nil
}

func (i *Infra) latestAMI(ctx context.Context) (string, error) {
	out, err := i.SSM.GetParameter(ctx, &ssm.GetParameterInput{
		Name: aws.String("/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-x86_64"),
	})
	if err != nil {
		return "", err
	}
	return aws.ToString(out.Parameter.Value), nil
}

func (i *Infra) defaultVPC(ctx context.Context) (string, error) {
	out, err := i.EC2.DescribeVpcs(ctx, &ec2.DescribeVpcsInput{
		Filters: []types.Filter{
			{Name: aws.String("is-default"), Values: []string{"true"}},
		},
	})
	if err != nil {
		return "", err
	}
	if len(out.Vpcs) == 0 {
		return "", fmt.Errorf("no default VPC found in %s", i.Region)
	}
	return aws.ToString(out.Vpcs[0].VpcId), nil
}

func tcpIngress(port int32, cidr, desc string) types.IpPermission {
	return types.IpPermission{
		IpProtocol: aws.String("tcp"),
		FromPort:   aws.Int32(port),
		ToPort:     aws.Int32(port),
		IpRanges:   []types.IpRange{{CidrIp: aws.String(cidr), Description: aws.String(desc)}},
	}
}

func (i *Infra) Init(ctx context.Context, pemDir string) (string, error) {
	// TODO: cleanup on error

	ami, err := i.latestAMI(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get latest AMI: %w", err)
	}

	vpcID, err := i.defaultVPC(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get default VPC: %w", err)
	}

	ip, err := myPublicIP(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get public IP: %w", err)
	}

	keyName := "zeroku-" + i.Region
	pemPath := filepath.Join(pemDir, keyName+".pem")
	if err := i.createKeyPair(ctx, keyName, pemPath); err != nil {
		return "", fmt.Errorf("failed to create key pair: %w", err)
	}

	sgID, err := i.createSecurityGroup(ctx, "zeroku-sg", vpcID, ip+"/32")
	if err != nil {
		return "", fmt.Errorf("failed to create security group: %w", err)
	}

	out, err := i.EC2.RunInstances(ctx, &ec2.RunInstancesInput{
		ImageId:          aws.String(ami),
		InstanceType:     types.InstanceTypeT3Micro,
		MinCount:         aws.Int32(1),
		MaxCount:         aws.Int32(1),
		KeyName:          aws.String(keyName),
		SecurityGroupIds: []string{sgID},
		UserData:         aws.String(base64.StdEncoding.EncodeToString([]byte(UserData))),
		MetadataOptions: &types.InstanceMetadataOptionsRequest{
			HttpTokens: types.HttpTokensStateRequired, // IMDSv2 only
		},
		BlockDeviceMappings: []types.BlockDeviceMapping{{
			DeviceName: aws.String("/dev/xvda"),
			Ebs: &types.EbsBlockDevice{
				VolumeSize:          aws.Int32(20),
				VolumeType:          types.VolumeTypeGp3,
				DeleteOnTermination: aws.Bool(true),
			},
		}},
		TagSpecifications: tags(types.ResourceTypeInstance, "zeroku"),
	})

	if err != nil {
		return "", fmt.Errorf("failed to run instance: %w", err)
	}

	if len(out.Instances) == 0 {
		return "", fmt.Errorf("no instances were created")
	}

	instanceID := aws.ToString(out.Instances[0].InstanceId)
	if instanceID == "" {
		return "", fmt.Errorf("instance ID is empty")
	}

	return instanceID, nil
}
