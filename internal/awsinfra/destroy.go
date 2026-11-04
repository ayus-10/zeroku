package awsinfra

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

func (i *Infra) terminateInstance(ctx context.Context, id string) error {
	_, err := i.EC2.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []string{id},
	})
	if err != nil {
		return err
	}
	w := ec2.NewInstanceTerminatedWaiter(i.EC2)
	return w.Wait(ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{id}}, 5*time.Minute)
}

func (i *Infra) releaseAddress(ctx context.Context, allocID string) error {
	_, err := i.EC2.ReleaseAddress(ctx, &ec2.ReleaseAddressInput{
		AllocationId: aws.String(allocID),
	})
	return err
}

func (i *Infra) deleteKeyPair(ctx context.Context, name, pemPath string) error {
	_, err := i.EC2.DeleteKeyPair(ctx, &ec2.DeleteKeyPairInput{
		KeyName: aws.String(name),
	})
	if pemPath != "" {
		os.Remove(pemPath)
	}
	return err
}

func (i *Infra) deleteSecurityGroup(ctx context.Context, sgID string) error {
	var err error
	for range 10 {
		_, err = i.EC2.DeleteSecurityGroup(ctx, &ec2.DeleteSecurityGroupInput{
			GroupId: aws.String(sgID),
		})
		if err == nil {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return err
}

func (i *Infra) Destroy(ctx context.Context, r *Resources) {
	if r.InstanceID != "" {
		if err := i.terminateInstance(ctx, r.InstanceID); err != nil {
			fmt.Println("warning: terminate instance:", err)
		}
	}
	if r.AllocationID != "" {
		if err := i.releaseAddress(ctx, r.AllocationID); err != nil {
			fmt.Println("warning: release address:", err)
		}
	}
	if r.SGID != "" {
		if err := i.deleteSecurityGroup(ctx, r.SGID); err != nil {
			fmt.Println("warning: delete security group:", err)
		}
	}
	if r.KeyName != "" {
		if err := i.deleteKeyPair(ctx, r.KeyName, r.PemPath); err != nil {
			fmt.Println("warning: delete key pair:", err)
		}
	}
}
