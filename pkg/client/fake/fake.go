package fake

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/jhjaggars/capa-annotator/pkg/client"
	"k8s.io/client-go/kubernetes"
)

type awsClient struct {
}

func (c *awsClient) DescribeImages(ctx context.Context, input *ec2.DescribeImagesInput) (*ec2.DescribeImagesOutput, error) {
	return &ec2.DescribeImagesOutput{
		Images: []types.Image{
			{
				ImageId: aws.String("ami-a9acbbd6"),
			},
		},
	}, nil
}

func (c *awsClient) DescribeVpcs(ctx context.Context, input *ec2.DescribeVpcsInput) (*ec2.DescribeVpcsOutput, error) {
	return &ec2.DescribeVpcsOutput{}, nil
}

func (c *awsClient) DescribeSubnets(ctx context.Context, input *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error) {
	return &ec2.DescribeSubnetsOutput{
		Subnets: []types.Subnet{
			{
				SubnetId: aws.String("subnet-28fddb3c45cae61b5"),
			},
		},
	}, nil
}

func (c *awsClient) DescribeAvailabilityZones(ctx context.Context, input *ec2.DescribeAvailabilityZonesInput) (*ec2.DescribeAvailabilityZonesOutput, error) {
	return &ec2.DescribeAvailabilityZonesOutput{}, nil
}

func (c *awsClient) DescribeSecurityGroups(ctx context.Context, input *ec2.DescribeSecurityGroupsInput) (*ec2.DescribeSecurityGroupsOutput, error) {
	return &ec2.DescribeSecurityGroupsOutput{
		SecurityGroups: []types.SecurityGroup{
			{
				GroupId: aws.String("sg-05acc3c38a35ce63b"),
			},
		},
	}, nil
}

func (c *awsClient) DescribePlacementGroups(ctx context.Context, input *ec2.DescribePlacementGroupsInput) (*ec2.DescribePlacementGroupsOutput, error) {
	return &ec2.DescribePlacementGroupsOutput{}, nil
}

func (c *awsClient) DescribeDHCPOptions(ctx context.Context, input *ec2.DescribeDhcpOptionsInput) (*ec2.DescribeDhcpOptionsOutput, error) {
	return &ec2.DescribeDhcpOptionsOutput{}, nil
}

func (c *awsClient) RunInstances(ctx context.Context, input *ec2.RunInstancesInput) (*ec2.RunInstancesOutput, error) {
	return &ec2.RunInstancesOutput{}, nil
}

func (c *awsClient) DescribeInstances(ctx context.Context, input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{}, nil
}

func (c *awsClient) DescribeInstanceTypes(ctx context.Context, input *ec2.DescribeInstanceTypesInput) (*ec2.DescribeInstanceTypesOutput, error) {
	return &ec2.DescribeInstanceTypesOutput{
		InstanceTypes: []types.InstanceTypeInfo{
			{
				InstanceType: types.InstanceTypeA12xlarge,
				MemoryInfo: &types.MemoryInfo{
					SizeInMiB: aws.Int64(16384),
				},
				VCpuInfo: &types.VCpuInfo{
					DefaultVCpus: aws.Int32(8),
				},
				ProcessorInfo: &types.ProcessorInfo{
					SupportedArchitectures: []types.ArchitectureType{
						types.ArchitectureTypeX8664,
					},
				},
			},
			{
				InstanceType: types.InstanceTypeP216xlarge,
				MemoryInfo: &types.MemoryInfo{
					SizeInMiB: aws.Int64(749568),
				},
				VCpuInfo: &types.VCpuInfo{
					DefaultVCpus: aws.Int32(64),
				},
				GpuInfo: &types.GpuInfo{
					Gpus: []types.GpuDeviceInfo{
						{
							Name:         aws.String("K80"),
							Manufacturer: aws.String("NVIDIA"),
							Count:        aws.Int32(16),
							MemoryInfo: &types.GpuDeviceMemoryInfo{
								SizeInMiB: aws.Int32(12288),
							},
						},
					},
					TotalGpuMemoryInMiB: aws.Int32(196608),
				},
				ProcessorInfo: &types.ProcessorInfo{
					SupportedArchitectures: []types.ArchitectureType{
						types.ArchitectureTypeX8664,
					},
				},
			},
			{
				InstanceType: types.InstanceTypeM6g4xlarge,
				MemoryInfo: &types.MemoryInfo{
					SizeInMiB: aws.Int64(65536),
				},
				VCpuInfo: &types.VCpuInfo{
					DefaultVCpus: aws.Int32(16),
				},
				ProcessorInfo: &types.ProcessorInfo{
					SupportedArchitectures: []types.ArchitectureType{
						types.ArchitectureTypeArm64,
					},
				},
			},
			{
				// This instance type misses the specification of the CPU Architecture.
				InstanceType: types.InstanceTypeM6i8xlarge,
				MemoryInfo: &types.MemoryInfo{
					SizeInMiB: aws.Int64(131072),
				},
				VCpuInfo: &types.VCpuInfo{
					DefaultVCpus: aws.Int32(32),
				},
			},
			{
				// This instance type reports a wrong specification of the CPU Architecture.
				// Note: Using a valid enum type but treated as "wrong" for testing purposes
				InstanceType: types.InstanceType("m6h.8xlarge"), // Custom instance type for testing
				MemoryInfo: &types.MemoryInfo{
					SizeInMiB: aws.Int64(131072),
				},
				VCpuInfo: &types.VCpuInfo{
					DefaultVCpus: aws.Int32(32),
				},
				ProcessorInfo: &types.ProcessorInfo{
					SupportedArchitectures: []types.ArchitectureType{
						types.ArchitectureType("wrong-arch"), // Custom invalid value
					},
				},
			},
		},
	}, nil
}

func (c *awsClient) TerminateInstances(ctx context.Context, input *ec2.TerminateInstancesInput) (*ec2.TerminateInstancesOutput, error) {
	return &ec2.TerminateInstancesOutput{}, nil
}

func (c *awsClient) DescribeVolumes(ctx context.Context, input *ec2.DescribeVolumesInput) (*ec2.DescribeVolumesOutput, error) {
	return &ec2.DescribeVolumesOutput{}, nil
}

func (c *awsClient) CreateTags(ctx context.Context, input *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error) {
	return &ec2.CreateTagsOutput{}, nil
}

func (c *awsClient) CreatePlacementGroup(ctx context.Context, input *ec2.CreatePlacementGroupInput) (*ec2.CreatePlacementGroupOutput, error) {
	return &ec2.CreatePlacementGroupOutput{}, nil
}

func (c *awsClient) DeletePlacementGroup(ctx context.Context, input *ec2.DeletePlacementGroupInput) (*ec2.DeletePlacementGroupOutput, error) {
	return &ec2.DeletePlacementGroupOutput{}, nil
}

func (c *awsClient) RegisterInstancesWithLoadBalancer(ctx context.Context, input *elasticloadbalancing.RegisterInstancesWithLoadBalancerInput) (*elasticloadbalancing.RegisterInstancesWithLoadBalancerOutput, error) {
	return &elasticloadbalancing.RegisterInstancesWithLoadBalancerOutput{}, nil
}

func (c *awsClient) ELBv2DescribeLoadBalancers(ctx context.Context, input *elasticloadbalancingv2.DescribeLoadBalancersInput) (*elasticloadbalancingv2.DescribeLoadBalancersOutput, error) {
	return &elasticloadbalancingv2.DescribeLoadBalancersOutput{}, nil
}

func (c *awsClient) ELBv2DescribeTargetGroups(ctx context.Context, input *elasticloadbalancingv2.DescribeTargetGroupsInput) (*elasticloadbalancingv2.DescribeTargetGroupsOutput, error) {
	return &elasticloadbalancingv2.DescribeTargetGroupsOutput{}, nil
}

func (c *awsClient) ELBv2DescribeTargetHealth(ctx context.Context, input *elasticloadbalancingv2.DescribeTargetHealthInput) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error) {
	return &elasticloadbalancingv2.DescribeTargetHealthOutput{}, nil
}

func (c *awsClient) ELBv2RegisterTargets(ctx context.Context, input *elasticloadbalancingv2.RegisterTargetsInput) (*elasticloadbalancingv2.RegisterTargetsOutput, error) {
	return &elasticloadbalancingv2.RegisterTargetsOutput{}, nil
}

func (c *awsClient) ELBv2DeregisterTargets(ctx context.Context, input *elasticloadbalancingv2.DeregisterTargetsInput) (*elasticloadbalancingv2.DeregisterTargetsOutput, error) {
	return &elasticloadbalancingv2.DeregisterTargetsOutput{}, nil
}

// NewClient creates a fake AWS client for testing.
func NewClient(kubeClient kubernetes.Interface, secretName, namespace, region string) (client.Client, error) {
	return &awsClient{}, nil
}
