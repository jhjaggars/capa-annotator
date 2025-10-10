/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/jhjaggars/capa-annotator/pkg/version"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/elb/elbiface"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
)

//go:generate go run ../../vendor/github.com/golang/mock/mockgen -source=./client.go -destination=./mock/client_generated.go -package=mock

const (
	// awsRegionsCacheExpirationDuration is the duration for which the AWS regions cache is valid
	awsRegionsCacheExpirationDuration = time.Minute * 30
)

// AwsClientBuilderFuncType is function type for building aws client
type AwsClientBuilderFuncType func(client client.Client, secretName, namespace, region string, regionCache RegionCache) (Client, error)

// Client is a wrapper object for actual AWS SDK clients to allow for easier testing.
type Client interface {
	DescribeImages(*ec2.DescribeImagesInput) (*ec2.DescribeImagesOutput, error)
	DescribeDHCPOptions(input *ec2.DescribeDhcpOptionsInput) (*ec2.DescribeDhcpOptionsOutput, error)
	DescribeVpcs(*ec2.DescribeVpcsInput) (*ec2.DescribeVpcsOutput, error)
	DescribeSubnets(*ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error)
	DescribeAvailabilityZones(*ec2.DescribeAvailabilityZonesInput) (*ec2.DescribeAvailabilityZonesOutput, error)
	DescribeSecurityGroups(*ec2.DescribeSecurityGroupsInput) (*ec2.DescribeSecurityGroupsOutput, error)
	DescribePlacementGroups(*ec2.DescribePlacementGroupsInput) (*ec2.DescribePlacementGroupsOutput, error)
	DescribeInstanceTypes(*ec2.DescribeInstanceTypesInput) (*ec2.DescribeInstanceTypesOutput, error)
	RunInstances(*ec2.RunInstancesInput) (*ec2.Reservation, error)
	DescribeInstances(*ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error)
	TerminateInstances(*ec2.TerminateInstancesInput) (*ec2.TerminateInstancesOutput, error)
	DescribeVolumes(*ec2.DescribeVolumesInput) (*ec2.DescribeVolumesOutput, error)
	CreateTags(*ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error)
	CreatePlacementGroup(*ec2.CreatePlacementGroupInput) (*ec2.CreatePlacementGroupOutput, error)
	DeletePlacementGroup(*ec2.DeletePlacementGroupInput) (*ec2.DeletePlacementGroupOutput, error)

	RegisterInstancesWithLoadBalancer(*elb.RegisterInstancesWithLoadBalancerInput) (*elb.RegisterInstancesWithLoadBalancerOutput, error)
	ELBv2DescribeLoadBalancers(*elbv2.DescribeLoadBalancersInput) (*elbv2.DescribeLoadBalancersOutput, error)
	ELBv2DescribeTargetGroups(*elbv2.DescribeTargetGroupsInput) (*elbv2.DescribeTargetGroupsOutput, error)
	ELBv2DescribeTargetHealth(*elbv2.DescribeTargetHealthInput) (*elbv2.DescribeTargetHealthOutput, error)
	ELBv2RegisterTargets(*elbv2.RegisterTargetsInput) (*elbv2.RegisterTargetsOutput, error)
	ELBv2DeregisterTargets(*elbv2.DeregisterTargetsInput) (*elbv2.DeregisterTargetsOutput, error)
}

type awsClient struct {
	ec2Client   ec2iface.EC2API
	elbClient   elbiface.ELBAPI
	elbv2Client elbv2iface.ELBV2API
}

func (c *awsClient) DescribeDHCPOptions(input *ec2.DescribeDhcpOptionsInput) (*ec2.DescribeDhcpOptionsOutput, error) {
	return c.ec2Client.DescribeDhcpOptions(input)
}

func (c *awsClient) DescribeImages(input *ec2.DescribeImagesInput) (*ec2.DescribeImagesOutput, error) {
	return c.ec2Client.DescribeImages(input)
}

func (c *awsClient) DescribeVpcs(input *ec2.DescribeVpcsInput) (*ec2.DescribeVpcsOutput, error) {
	return c.ec2Client.DescribeVpcs(input)
}

func (c *awsClient) DescribeSubnets(input *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error) {
	return c.ec2Client.DescribeSubnets(input)
}

func (c *awsClient) DescribeAvailabilityZones(input *ec2.DescribeAvailabilityZonesInput) (*ec2.DescribeAvailabilityZonesOutput, error) {
	return c.ec2Client.DescribeAvailabilityZones(input)
}

func (c *awsClient) DescribeSecurityGroups(input *ec2.DescribeSecurityGroupsInput) (*ec2.DescribeSecurityGroupsOutput, error) {
	return c.ec2Client.DescribeSecurityGroups(input)
}

func (c *awsClient) DescribePlacementGroups(input *ec2.DescribePlacementGroupsInput) (*ec2.DescribePlacementGroupsOutput, error) {
	return c.ec2Client.DescribePlacementGroups(input)
}

func (c *awsClient) DescribeInstanceTypes(input *ec2.DescribeInstanceTypesInput) (*ec2.DescribeInstanceTypesOutput, error) {
	return c.ec2Client.DescribeInstanceTypes(input)
}

func (c *awsClient) RunInstances(input *ec2.RunInstancesInput) (*ec2.Reservation, error) {
	return c.ec2Client.RunInstances(input)
}

func (c *awsClient) DescribeInstances(input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	return c.ec2Client.DescribeInstances(input)
}

func (c *awsClient) TerminateInstances(input *ec2.TerminateInstancesInput) (*ec2.TerminateInstancesOutput, error) {
	return c.ec2Client.TerminateInstances(input)
}

func (c *awsClient) DescribeVolumes(input *ec2.DescribeVolumesInput) (*ec2.DescribeVolumesOutput, error) {
	return c.ec2Client.DescribeVolumes(input)
}

func (c *awsClient) CreateTags(input *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error) {
	return c.ec2Client.CreateTags(input)
}

func (c *awsClient) CreatePlacementGroup(input *ec2.CreatePlacementGroupInput) (*ec2.CreatePlacementGroupOutput, error) {
	return c.ec2Client.CreatePlacementGroup(input)
}

func (c *awsClient) DeletePlacementGroup(input *ec2.DeletePlacementGroupInput) (*ec2.DeletePlacementGroupOutput, error) {
	return c.ec2Client.DeletePlacementGroup(input)
}

func (c *awsClient) RegisterInstancesWithLoadBalancer(input *elb.RegisterInstancesWithLoadBalancerInput) (*elb.RegisterInstancesWithLoadBalancerOutput, error) {
	return c.elbClient.RegisterInstancesWithLoadBalancer(input)
}

func (c *awsClient) ELBv2DescribeLoadBalancers(input *elbv2.DescribeLoadBalancersInput) (*elbv2.DescribeLoadBalancersOutput, error) {
	return c.elbv2Client.DescribeLoadBalancers(input)
}

func (c *awsClient) ELBv2DescribeTargetGroups(input *elbv2.DescribeTargetGroupsInput) (*elbv2.DescribeTargetGroupsOutput, error) {
	return c.elbv2Client.DescribeTargetGroups(input)
}

func (c *awsClient) ELBv2DescribeTargetHealth(input *elbv2.DescribeTargetHealthInput) (*elbv2.DescribeTargetHealthOutput, error) {
	return c.elbv2Client.DescribeTargetHealth(input)
}

func (c *awsClient) ELBv2RegisterTargets(input *elbv2.RegisterTargetsInput) (*elbv2.RegisterTargetsOutput, error) {
	return c.elbv2Client.RegisterTargets(input)
}

func (c *awsClient) ELBv2DeregisterTargets(input *elbv2.DeregisterTargetsInput) (*elbv2.DeregisterTargetsOutput, error) {
	return c.elbv2Client.DeregisterTargets(input)
}

// NewClient creates our client wrapper object for the actual AWS clients we use.
// For authentication the underlying clients will use IRSA (IAM Roles for Service Accounts).
func NewClient(ctrlRuntimeClient client.Client, secretName, namespace, region string) (Client, error) {
	s, err := newAWSSession(ctrlRuntimeClient, secretName, namespace, region)
	if err != nil {
		return nil, err
	}

	return &awsClient{
		ec2Client:   ec2.New(s),
		elbClient:   elb.New(s),
		elbv2Client: elbv2.New(s),
	}, nil
}

// NewClientFromKeys creates our client wrapper object for the actual AWS clients we use.
// For authentication the underlying clients will use AWS credentials.
func NewClientFromKeys(accessKey, secretAccessKey, region string) (Client, error) {
	awsConfig := &aws.Config{
		Region: aws.String(region),
		Credentials: credentials.NewStaticCredentials(
			accessKey,
			secretAccessKey,
			"",
		),
	}

	s, err := session.NewSession(awsConfig)
	if err != nil {
		return nil, err
	}
	s.Handlers.Build.PushBackNamed(addProviderVersionToUserAgent)

	return &awsClient{
		ec2Client:   ec2.New(s),
		elbClient:   elb.New(s),
		elbv2Client: elbv2.New(s),
	}, nil
}

// DescribeRegionsData holds output of DescribeRegions API call and the time when it was last updated.
type DescribeRegionsData struct {
	err                   error
	describeRegionsOutput *ec2.DescribeRegionsOutput
	lastUpdated           time.Time
}

type regionCache struct {
	data  map[string]DescribeRegionsData
	mutex sync.RWMutex
}

// RegionCache caches successful DescribeRegions API calls.
type RegionCache interface {
	GetCachedDescribeRegions(awsSession *session.Session) (*ec2.DescribeRegionsOutput, error)
}

// NewRegionCache creates a new empty DescribeRegionsData cache with lock.
func NewRegionCache() RegionCache {
	return &regionCache{
		data:  map[string]DescribeRegionsData{},
		mutex: sync.RWMutex{},
	}
}

// GetCachedDescribeRegions returns DescribeRegionsOutput from DescribeRegions AWS API call.
// It is cached to avoid AWS API calls on each reconcile loop.
func (c *regionCache) GetCachedDescribeRegions(awsSession *session.Session) (*ec2.DescribeRegionsOutput, error) {
	creds, err := awsSession.Config.Credentials.Get()
	if err != nil {
		return nil, err
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()
	regionData := c.data[creds.AccessKeyID]
	if regionData.describeRegionsOutput != nil && regionData.err == nil &&
		time.Since(regionData.lastUpdated) < awsRegionsCacheExpirationDuration {
		klog.Info("Using cached AWS region data")
		return regionData.describeRegionsOutput, nil
	}

	currentRegion := awsSession.Config.Region
	// Use default region to send our request
	awsSession.Config.Region = aws.String("us-east-1")
	describeRegionsOutput, err := ec2.New(awsSession).DescribeRegions(&ec2.DescribeRegionsInput{
		AllRegions: aws.Bool(true),
		DryRun:     aws.Bool(false),
	})
	// Restore the original region
	awsSession.Config.Region = currentRegion
	if err != nil {
		regionData.err = err
		return nil, err
	}

	regionData.describeRegionsOutput = describeRegionsOutput
	regionData.lastUpdated = time.Now()
	c.data[creds.AccessKeyID] = regionData
	return describeRegionsOutput, nil
}

// Check that region is in the DescribeRegions list and is opted in.
func validateRegion(describeRegionsOutput *ec2.DescribeRegionsOutput, region string) (*ec2.Region, error) {
	var regionData *ec2.Region
	for _, currentRegion := range describeRegionsOutput.Regions {
		if currentRegion != nil && *currentRegion.RegionName == region {
			regionData = currentRegion
			break
		}
	}

	if regionData == nil {
		return nil, fmt.Errorf("region %s is not a valid region", region)
	}
	if *regionData.OptInStatus == "not-opted-in" {
		return nil, fmt.Errorf("region %s is not opted in", region)
	}
	klog.Infof("AWS reports region %s is valid", region)
	return regionData, nil
}

// NewValidatedClient creates our client wrapper object for the actual AWS clients we use.
// This should behave the same as NewClient except it will validate the client configuration
// (eg the region) before returning the client.
func NewValidatedClient(ctrlRuntimeClient client.Client, secretName, namespace, region string, regionCache RegionCache) (Client, error) {
	s, err := newAWSSession(ctrlRuntimeClient, secretName, namespace, region)
	if err != nil {
		return nil, err
	}

	// Check that the endpoint can be resolved by the endpoint resolver.
	// If the endpoint is not resolvable locally, we try to validate using the AWS API.
	// If the endpoint is not known, it is not a standard or configured custom region.
	// In that case, the client will likely not be able to connect
	_, err = s.Config.EndpointResolver.EndpointFor("ec2", region, func(opts *endpoints.Options) {
		opts.StrictMatching = true
	})
	if err != nil {
		switch err.(type) {
		case endpoints.UnknownEndpointError:
			klog.Infof("Region %s is not recognized by aws-sdk, trying to validate using API", region)
			var describeRegionsOutput *ec2.DescribeRegionsOutput
			describeRegionsOutput, err = regionCache.GetCachedDescribeRegions(s)
			if err != nil {
				return nil, fmt.Errorf("could not retrieve region data: %w", err)
			}

			_, err = validateRegion(describeRegionsOutput, region)
			if err != nil {
				return nil, err
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("region %q not resolved: %w", region, err)
	}

	return &awsClient{
		ec2Client:   ec2.New(s),
		elbClient:   elb.New(s),
		elbv2Client: elbv2.New(s),
	}, nil
}

func newAWSSession(ctrlRuntimeClient client.Client, secretName, namespace, region string) (*session.Session, error) {
	sessionOptions := session.Options{
		Config: aws.Config{
			Region: aws.String(region),
		},
	}

	// Check for IRSA environment variables
	roleARN := os.Getenv("AWS_ROLE_ARN")
	tokenFile := os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE")

	// Prefer IRSA if configured, otherwise fall back to default credential chain
	// This allows local testing with ~/.aws/credentials or environment variables
	if roleARN != "" && tokenFile != "" {
		klog.Infof("Using IRSA authentication with role: %s", roleARN)
		// AWS SDK v1 will automatically detect and use web identity credentials
		// from the environment variables - no explicit configuration needed
	} else {
		klog.Info("IRSA not configured, using default AWS credential chain (environment variables, ~/.aws/credentials, EC2 metadata, etc.)")
		// AWS SDK will use the default credential chain:
		// 1. Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
		// 2. Shared credentials file (~/.aws/credentials)
		// 3. EC2 instance metadata
	}

	// Create AWS session with the configured options
	s, err := session.NewSessionWithOptions(sessionOptions)
	if err != nil {
		return nil, err
	}

	s.Handlers.Build.PushBackNamed(addProviderVersionToUserAgent)

	return s, nil
}

// addProviderVersionToUserAgent is a named handler that will add cluster-api-provider-aws
// version information to requests made by the AWS SDK.
var addProviderVersionToUserAgent = request.NamedHandler{
	Name: "capa-annotator",
	Fn:   request.MakeAddToUserAgentHandler("github.com/jhjaggars capa-annotator", version.Version),
}
