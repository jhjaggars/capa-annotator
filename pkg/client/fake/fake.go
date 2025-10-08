package fake

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/jhjaggars/capa-annotator/pkg/client"
	"k8s.io/client-go/kubernetes"
)

type awsClient struct {
}

func (c *awsClient) DescribeImages(input *ec2.DescribeImagesInput) (*ec2.DescribeImagesOutput, error) {
	return &ec2.DescribeImagesOutput{
		Images: []*ec2.Image{
			{
				ImageId: aws.String("ami-a9acbbd6"),
			},
		},
	}, nil
}

func (c *awsClient) DescribeVpcs(input *ec2.DescribeVpcsInput) (*ec2.DescribeVpcsOutput, error) {
	return &ec2.DescribeVpcsOutput{}, nil
}

func (c *awsClient) DescribeSubnets(input *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error) {
	return &ec2.DescribeSubnetsOutput{
		Subnets: []*ec2.Subnet{
			{
				SubnetId: aws.String("subnet-28fddb3c45cae61b5"),
			},
		},
	}, nil
}

func (c *awsClient) DescribeAvailabilityZones(*ec2.DescribeAvailabilityZonesInput) (*ec2.DescribeAvailabilityZonesOutput, error) {
	return &ec2.DescribeAvailabilityZonesOutput{}, nil
}

func (c *awsClient) DescribeSecurityGroups(input *ec2.DescribeSecurityGroupsInput) (*ec2.DescribeSecurityGroupsOutput, error) {
	return &ec2.DescribeSecurityGroupsOutput{
		SecurityGroups: []*ec2.SecurityGroup{
			{
				GroupId: aws.String("sg-05acc3c38a35ce63b"),
			},
		},
	}, nil
}

func (c *awsClient) DescribePlacementGroups(*ec2.DescribePlacementGroupsInput) (*ec2.DescribePlacementGroupsOutput, error) {
	return &ec2.DescribePlacementGroupsOutput{}, nil
}

func (c *awsClient) DescribeDHCPOptions(input *ec2.DescribeDhcpOptionsInput) (*ec2.DescribeDhcpOptionsOutput, error) {
	return &ec2.DescribeDhcpOptionsOutput{}, nil
}

func (c *awsClient) RunInstances(input *ec2.RunInstancesInput) (*ec2.Reservation, error) {
	return &ec2.Reservation{}, nil
}

func (c *awsClient) DescribeInstances(input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{}, nil
}

func (c *awsClient) DescribeInstanceTypes(input *ec2.DescribeInstanceTypesInput) (*ec2.DescribeInstanceTypesOutput, error) {
	return &ec2.DescribeInstanceTypesOutput{
		InstanceTypes: []*ec2.InstanceTypeInfo{
			{
				InstanceType: aws.String("a1.2xlarge"),
				MemoryInfo: &ec2.MemoryInfo{
					SizeInMiB: aws.Int64(16384),
				},
				VCpuInfo: &ec2.VCpuInfo{
					DefaultVCpus: aws.Int64(8),
				},
				ProcessorInfo: &ec2.ProcessorInfo{
					SupportedArchitectures: []*string{
						aws.String("amd64"),
					},
				},
			},
			{
				InstanceType: aws.String("p2.16xlarge"),
				MemoryInfo: &ec2.MemoryInfo{
					SizeInMiB: aws.Int64(749568),
				},
				VCpuInfo: &ec2.VCpuInfo{
					DefaultVCpus: aws.Int64(64),
				},
				GpuInfo: &ec2.GpuInfo{
					Gpus: []*ec2.GpuDeviceInfo{
						{
							Name:         aws.String("K80"),
							Manufacturer: aws.String("NVIDIA"),
							Count:        aws.Int64(16),
							MemoryInfo: &ec2.GpuDeviceMemoryInfo{
								SizeInMiB: aws.Int64(12288),
							},
						},
					},
					TotalGpuMemoryInMiB: aws.Int64(196608),
				},
				ProcessorInfo: &ec2.ProcessorInfo{
					SupportedArchitectures: []*string{
						aws.String("amd64"),
					},
				},
			},
			{
				InstanceType: aws.String("m6g.4xlarge"),
				MemoryInfo: &ec2.MemoryInfo{
					SizeInMiB: aws.Int64(65536),
				},
				VCpuInfo: &ec2.VCpuInfo{
					DefaultVCpus: aws.Int64(16),
				},
				ProcessorInfo: &ec2.ProcessorInfo{
					SupportedArchitectures: []*string{
						aws.String("arm64"),
					},
				},
			},
			{
				// This instance type misses the specification of the CPU Architecture.
				InstanceType: aws.String("m6i.8xlarge"),
				MemoryInfo: &ec2.MemoryInfo{
					SizeInMiB: aws.Int64(131072),
				},
				VCpuInfo: &ec2.VCpuInfo{
					DefaultVCpus: aws.Int64(32),
				},
			},
			{
				// This instance type reports a wrong specification of the CPU Architecture.
				InstanceType: aws.String("m6h.8xlarge"),
				MemoryInfo: &ec2.MemoryInfo{
					SizeInMiB: aws.Int64(131072),
				},
				VCpuInfo: &ec2.VCpuInfo{
					DefaultVCpus: aws.Int64(32),
				},
				ProcessorInfo: &ec2.ProcessorInfo{
					SupportedArchitectures: []*string{
						aws.String("wrong-arch"),
					},
				},
			},
		},
	}, nil
}

func (c *awsClient) TerminateInstances(input *ec2.TerminateInstancesInput) (*ec2.TerminateInstancesOutput, error) {
	return &ec2.TerminateInstancesOutput{}, nil
}

func (c *awsClient) DescribeVolumes(input *ec2.DescribeVolumesInput) (*ec2.DescribeVolumesOutput, error) {
	return &ec2.DescribeVolumesOutput{}, nil
}

func (c *awsClient) CreateTags(input *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error) {
	return &ec2.CreateTagsOutput{}, nil
}

func (c *awsClient) CreatePlacementGroup(input *ec2.CreatePlacementGroupInput) (*ec2.CreatePlacementGroupOutput, error) {
	return &ec2.CreatePlacementGroupOutput{}, nil
}

func (c *awsClient) DeletePlacementGroup(input *ec2.DeletePlacementGroupInput) (*ec2.DeletePlacementGroupOutput, error) {
	return &ec2.DeletePlacementGroupOutput{}, nil
}

func (c *awsClient) RegisterInstancesWithLoadBalancer(input *elb.RegisterInstancesWithLoadBalancerInput) (*elb.RegisterInstancesWithLoadBalancerOutput, error) {
	return &elb.RegisterInstancesWithLoadBalancerOutput{}, nil
}

func (c *awsClient) ELBv2DescribeLoadBalancers(*elbv2.DescribeLoadBalancersInput) (*elbv2.DescribeLoadBalancersOutput, error) {
	return &elbv2.DescribeLoadBalancersOutput{}, nil
}

func (c *awsClient) ELBv2DescribeTargetGroups(*elbv2.DescribeTargetGroupsInput) (*elbv2.DescribeTargetGroupsOutput, error) {
	return &elbv2.DescribeTargetGroupsOutput{}, nil
}

func (c *awsClient) ELBv2DescribeTargetHealth(*elbv2.DescribeTargetHealthInput) (*elbv2.DescribeTargetHealthOutput, error) {
	return &elbv2.DescribeTargetHealthOutput{}, nil
}

func (c *awsClient) ELBv2RegisterTargets(*elbv2.RegisterTargetsInput) (*elbv2.RegisterTargetsOutput, error) {
	return &elbv2.RegisterTargetsOutput{}, nil
}

func (c *awsClient) ELBv2DeregisterTargets(*elbv2.DeregisterTargetsInput) (*elbv2.DeregisterTargetsOutput, error) {
	return &elbv2.DeregisterTargetsOutput{}, nil
}

// NewClient creates a fake AWS client for testing.
func NewClient(kubeClient kubernetes.Interface, secretName, namespace, region string) (client.Client, error) {
	return &awsClient{}, nil
}
