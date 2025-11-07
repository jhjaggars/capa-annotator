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
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/jhjaggars/capa-annotator/pkg/version"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
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
	DescribeImages(context.Context, *ec2.DescribeImagesInput) (*ec2.DescribeImagesOutput, error)
	DescribeDHCPOptions(context.Context, *ec2.DescribeDhcpOptionsInput) (*ec2.DescribeDhcpOptionsOutput, error)
	DescribeVpcs(context.Context, *ec2.DescribeVpcsInput) (*ec2.DescribeVpcsOutput, error)
	DescribeSubnets(context.Context, *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error)
	DescribeAvailabilityZones(context.Context, *ec2.DescribeAvailabilityZonesInput) (*ec2.DescribeAvailabilityZonesOutput, error)
	DescribeSecurityGroups(context.Context, *ec2.DescribeSecurityGroupsInput) (*ec2.DescribeSecurityGroupsOutput, error)
	DescribePlacementGroups(context.Context, *ec2.DescribePlacementGroupsInput) (*ec2.DescribePlacementGroupsOutput, error)
	DescribeInstanceTypes(context.Context, *ec2.DescribeInstanceTypesInput) (*ec2.DescribeInstanceTypesOutput, error)
	RunInstances(context.Context, *ec2.RunInstancesInput) (*ec2.RunInstancesOutput, error)
	DescribeInstances(context.Context, *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error)
	TerminateInstances(context.Context, *ec2.TerminateInstancesInput) (*ec2.TerminateInstancesOutput, error)
	DescribeVolumes(context.Context, *ec2.DescribeVolumesInput) (*ec2.DescribeVolumesOutput, error)
	CreateTags(context.Context, *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error)
	CreatePlacementGroup(context.Context, *ec2.CreatePlacementGroupInput) (*ec2.CreatePlacementGroupOutput, error)
	DeletePlacementGroup(context.Context, *ec2.DeletePlacementGroupInput) (*ec2.DeletePlacementGroupOutput, error)

	RegisterInstancesWithLoadBalancer(context.Context, *elasticloadbalancing.RegisterInstancesWithLoadBalancerInput) (*elasticloadbalancing.RegisterInstancesWithLoadBalancerOutput, error)
	ELBv2DescribeLoadBalancers(context.Context, *elasticloadbalancingv2.DescribeLoadBalancersInput) (*elasticloadbalancingv2.DescribeLoadBalancersOutput, error)
	ELBv2DescribeTargetGroups(context.Context, *elasticloadbalancingv2.DescribeTargetGroupsInput) (*elasticloadbalancingv2.DescribeTargetGroupsOutput, error)
	ELBv2DescribeTargetHealth(context.Context, *elasticloadbalancingv2.DescribeTargetHealthInput) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error)
	ELBv2RegisterTargets(context.Context, *elasticloadbalancingv2.RegisterTargetsInput) (*elasticloadbalancingv2.RegisterTargetsOutput, error)
	ELBv2DeregisterTargets(context.Context, *elasticloadbalancingv2.DeregisterTargetsInput) (*elasticloadbalancingv2.DeregisterTargetsOutput, error)
}

type awsClient struct {
	ec2Client   *ec2.Client
	elbClient   *elasticloadbalancing.Client
	elbv2Client *elasticloadbalancingv2.Client
}

func (c *awsClient) DescribeDHCPOptions(ctx context.Context, input *ec2.DescribeDhcpOptionsInput) (*ec2.DescribeDhcpOptionsOutput, error) {
	return c.ec2Client.DescribeDhcpOptions(ctx, input)
}

func (c *awsClient) DescribeImages(ctx context.Context, input *ec2.DescribeImagesInput) (*ec2.DescribeImagesOutput, error) {
	return c.ec2Client.DescribeImages(ctx, input)
}

func (c *awsClient) DescribeVpcs(ctx context.Context, input *ec2.DescribeVpcsInput) (*ec2.DescribeVpcsOutput, error) {
	return c.ec2Client.DescribeVpcs(ctx, input)
}

func (c *awsClient) DescribeSubnets(ctx context.Context, input *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error) {
	return c.ec2Client.DescribeSubnets(ctx, input)
}

func (c *awsClient) DescribeAvailabilityZones(ctx context.Context, input *ec2.DescribeAvailabilityZonesInput) (*ec2.DescribeAvailabilityZonesOutput, error) {
	return c.ec2Client.DescribeAvailabilityZones(ctx, input)
}

func (c *awsClient) DescribeSecurityGroups(ctx context.Context, input *ec2.DescribeSecurityGroupsInput) (*ec2.DescribeSecurityGroupsOutput, error) {
	return c.ec2Client.DescribeSecurityGroups(ctx, input)
}

func (c *awsClient) DescribePlacementGroups(ctx context.Context, input *ec2.DescribePlacementGroupsInput) (*ec2.DescribePlacementGroupsOutput, error) {
	return c.ec2Client.DescribePlacementGroups(ctx, input)
}

func (c *awsClient) DescribeInstanceTypes(ctx context.Context, input *ec2.DescribeInstanceTypesInput) (*ec2.DescribeInstanceTypesOutput, error) {
	return c.ec2Client.DescribeInstanceTypes(ctx, input)
}

func (c *awsClient) RunInstances(ctx context.Context, input *ec2.RunInstancesInput) (*ec2.RunInstancesOutput, error) {
	return c.ec2Client.RunInstances(ctx, input)
}

func (c *awsClient) DescribeInstances(ctx context.Context, input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	return c.ec2Client.DescribeInstances(ctx, input)
}

func (c *awsClient) TerminateInstances(ctx context.Context, input *ec2.TerminateInstancesInput) (*ec2.TerminateInstancesOutput, error) {
	return c.ec2Client.TerminateInstances(ctx, input)
}

func (c *awsClient) DescribeVolumes(ctx context.Context, input *ec2.DescribeVolumesInput) (*ec2.DescribeVolumesOutput, error) {
	return c.ec2Client.DescribeVolumes(ctx, input)
}

func (c *awsClient) CreateTags(ctx context.Context, input *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error) {
	return c.ec2Client.CreateTags(ctx, input)
}

func (c *awsClient) CreatePlacementGroup(ctx context.Context, input *ec2.CreatePlacementGroupInput) (*ec2.CreatePlacementGroupOutput, error) {
	return c.ec2Client.CreatePlacementGroup(ctx, input)
}

func (c *awsClient) DeletePlacementGroup(ctx context.Context, input *ec2.DeletePlacementGroupInput) (*ec2.DeletePlacementGroupOutput, error) {
	return c.ec2Client.DeletePlacementGroup(ctx, input)
}

func (c *awsClient) RegisterInstancesWithLoadBalancer(ctx context.Context, input *elasticloadbalancing.RegisterInstancesWithLoadBalancerInput) (*elasticloadbalancing.RegisterInstancesWithLoadBalancerOutput, error) {
	return c.elbClient.RegisterInstancesWithLoadBalancer(ctx, input)
}

func (c *awsClient) ELBv2DescribeLoadBalancers(ctx context.Context, input *elasticloadbalancingv2.DescribeLoadBalancersInput) (*elasticloadbalancingv2.DescribeLoadBalancersOutput, error) {
	return c.elbv2Client.DescribeLoadBalancers(ctx, input)
}

func (c *awsClient) ELBv2DescribeTargetGroups(ctx context.Context, input *elasticloadbalancingv2.DescribeTargetGroupsInput) (*elasticloadbalancingv2.DescribeTargetGroupsOutput, error) {
	return c.elbv2Client.DescribeTargetGroups(ctx, input)
}

func (c *awsClient) ELBv2DescribeTargetHealth(ctx context.Context, input *elasticloadbalancingv2.DescribeTargetHealthInput) (*elasticloadbalancingv2.DescribeTargetHealthOutput, error) {
	return c.elbv2Client.DescribeTargetHealth(ctx, input)
}

func (c *awsClient) ELBv2RegisterTargets(ctx context.Context, input *elasticloadbalancingv2.RegisterTargetsInput) (*elasticloadbalancingv2.RegisterTargetsOutput, error) {
	return c.elbv2Client.RegisterTargets(ctx, input)
}

func (c *awsClient) ELBv2DeregisterTargets(ctx context.Context, input *elasticloadbalancingv2.DeregisterTargetsInput) (*elasticloadbalancingv2.DeregisterTargetsOutput, error) {
	return c.elbv2Client.DeregisterTargets(ctx, input)
}

// NewClient creates our client wrapper object for the actual AWS clients we use.
// For authentication the underlying clients will use IRSA (IAM Roles for Service Accounts)
// or fall back to the default AWS credential chain.
// Note: secretName and namespace parameters are deprecated and unused (kept for API compatibility).
func NewClient(ctrlRuntimeClient client.Client, secretName, namespace, region string) (Client, error) {
	cfg, err := newAWSConfig(context.Background(), region)
	if err != nil {
		return nil, err
	}

	// Get custom endpoint URL for LocalStack/testing
	endpointURL := os.Getenv("AWS_ENDPOINT_URL")

	return &awsClient{
		ec2Client:   ec2.NewFromConfig(cfg, applyEndpointURL(endpointURL)),
		elbClient:   elasticloadbalancing.NewFromConfig(cfg, applyEndpointURLForELB(endpointURL)),
		elbv2Client: elasticloadbalancingv2.NewFromConfig(cfg, applyEndpointURLForELBv2(endpointURL)),
	}, nil
}

// NewClientFromKeys creates our client wrapper object for the actual AWS clients we use.
// For authentication the underlying clients will use AWS credentials.
func NewClientFromKeys(accessKey, secretAccessKey, region string) (Client, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKey,
			secretAccessKey,
			"",
		)),
		config.WithAPIOptions([]func(*middleware.Stack) error{
			addUserAgentMiddleware,
		}),
	)
	if err != nil {
		return nil, err
	}

	// Get custom endpoint URL for LocalStack/testing
	endpointURL := os.Getenv("AWS_ENDPOINT_URL")

	return &awsClient{
		ec2Client:   ec2.NewFromConfig(cfg, applyEndpointURL(endpointURL)),
		elbClient:   elasticloadbalancing.NewFromConfig(cfg, applyEndpointURLForELB(endpointURL)),
		elbv2Client: elasticloadbalancingv2.NewFromConfig(cfg, applyEndpointURLForELBv2(endpointURL)),
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
	GetCachedDescribeRegions(ctx context.Context, cfg aws.Config) (*ec2.DescribeRegionsOutput, error)
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
func (c *regionCache) GetCachedDescribeRegions(ctx context.Context, cfg aws.Config) (*ec2.DescribeRegionsOutput, error) {
	creds, err := cfg.Credentials.Retrieve(ctx)
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

	// Use a copy of the config to avoid mutating the original
	// AWS SDK v2 configs should be treated as immutable
	tempCfg := cfg.Copy()
	tempCfg.Region = "us-east-1"
	allRegions := true
	dryRun := false
	describeRegionsOutput, err := ec2.NewFromConfig(tempCfg).DescribeRegions(ctx, &ec2.DescribeRegionsInput{
		AllRegions: &allRegions,
		DryRun:     &dryRun,
	})
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
func validateRegion(describeRegionsOutput *ec2.DescribeRegionsOutput, region string) error {
	var regionData *types.Region
	for _, currentRegion := range describeRegionsOutput.Regions {
		if currentRegion.RegionName != nil && *currentRegion.RegionName == region {
			regionData = &currentRegion
			break
		}
	}

	if regionData == nil {
		return fmt.Errorf("region %s is not a valid region", region)
	}
	if regionData.OptInStatus != nil && *regionData.OptInStatus == "not-opted-in" {
		return fmt.Errorf("region %s is not opted in", region)
	}
	klog.Infof("AWS reports region %s is valid", region)
	return nil
}

// NewValidatedClient creates our client wrapper object for the actual AWS clients we use.
// This should behave the same as NewClient except it will validate the client configuration
// (eg the region) before returning the client.
// Note: ctrlRuntimeClient, secretName and namespace parameters are deprecated and unused (kept for API compatibility).
func NewValidatedClient(ctrlRuntimeClient client.Client, secretName, namespace, region string, regionCache RegionCache) (Client, error) {
	cfg, err := newAWSConfig(context.Background(), region)
	if err != nil {
		return nil, err
	}

	// Validate region using AWS API
	// Note: This is a behavior change from AWS SDK v1 to v2:
	// - v1: Used local endpoint resolver first, fell back to AWS API if endpoint was unknown
	// - v2: Always validates via AWS API (DescribeRegions) because v2 removed the endpoint resolver
	//
	// Implications:
	// - Every client creation will make an AWS API call (or use cached result with 30min TTL)
	// - Cold start: Adds ~200-500ms latency for DescribeRegions API call
	// - Warm cache: No additional latency
	// - Potential for transient network issues to affect client creation
	// - May encounter AWS API throttling if many clients are created rapidly
	//
	// This change was necessary for v2 migration and is mitigated by the 30-minute cache
	klog.Infof("Validating region %s using AWS API", region)
	describeRegionsOutput, err := regionCache.GetCachedDescribeRegions(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve region data: %w", err)
	}

	err = validateRegion(describeRegionsOutput, region)
	if err != nil {
		return nil, err
	}

	return &awsClient{
		ec2Client:   ec2.NewFromConfig(cfg),
		elbClient:   elasticloadbalancing.NewFromConfig(cfg),
		elbv2Client: elasticloadbalancingv2.NewFromConfig(cfg),
	}, nil
}

func newAWSConfig(ctx context.Context, region string) (aws.Config, error) {
	// Check for IRSA environment variables
	roleARN := os.Getenv("AWS_ROLE_ARN")
	tokenFile := os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE")
	endpointURL := os.Getenv("AWS_ENDPOINT_URL")

	// Prefer IRSA if configured, otherwise fall back to default credential chain
	// This allows local testing with ~/.aws/credentials or environment variables
	if roleARN != "" && tokenFile != "" {
		klog.Infof("Using IRSA authentication with role: %s", roleARN)
		// AWS SDK v2 will automatically detect and use web identity credentials
		// from the environment variables - no explicit configuration needed
	} else {
		klog.Info("IRSA not configured, using default AWS credential chain (environment variables, ~/.aws/credentials, EC2 metadata, etc.)")
		// AWS SDK will use the default credential chain:
		// 1. Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
		// 2. Shared credentials file (~/.aws/credentials)
		// 3. EC2 instance metadata
	}

	// Build config options
	configOptions := []func(*config.LoadOptions) error{
		config.WithRegion(region),
		config.WithAPIOptions([]func(*middleware.Stack) error{
			addUserAgentMiddleware,
		}),
	}

	// If using LocalStack or custom endpoint, configure it for all services including
	// the STS client used by credential providers (e.g., IRSA)
	if endpointURL != "" {
		klog.Infof("Configuring custom endpoint for all services: %s", endpointURL)
		// Note: EndpointResolverWithOptions is deprecated, but required for LocalStack IRSA.
		// The WebIdentityCredentials provider creates its own internal STS client that needs
		// global endpoint configuration. BaseEndpoint only works for service clients we create
		// directly. This is only used in testing scenarios where AWS_ENDPOINT_URL is set.
		//lint:ignore SA1019 Deprecated but required for LocalStack IRSA testing
		configOptions = append(configOptions, config.WithEndpointResolverWithOptions(
			//lint:ignore SA1019 Deprecated but required for LocalStack IRSA testing
			aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				//lint:ignore SA1019 Deprecated but required for LocalStack IRSA testing
				return aws.Endpoint{
					URL:           endpointURL,
					SigningRegion: region,
				}, nil
			}),
		))
	}

	// Create AWS config with the configured options
	cfg, err := config.LoadDefaultConfig(ctx, configOptions...)
	if err != nil {
		return aws.Config{}, err
	}

	return cfg, nil
}

// applyEndpointURL applies a custom endpoint URL to service client options if configured.
// This is used for LocalStack and other AWS-compatible services.
// The AWS_ENDPOINT_URL environment variable is automatically respected by the SDK's
// credential providers (including IRSA), so this only needs to be called for service clients.
func applyEndpointURL(endpointURL string) func(*ec2.Options) {
	return func(o *ec2.Options) {
		if endpointURL != "" {
			klog.Infof("Using custom AWS endpoint: %s", endpointURL)
			o.BaseEndpoint = aws.String(endpointURL)
		}
	}
}

func applyEndpointURLForELB(endpointURL string) func(*elasticloadbalancing.Options) {
	return func(o *elasticloadbalancing.Options) {
		if endpointURL != "" {
			o.BaseEndpoint = aws.String(endpointURL)
		}
	}
}

func applyEndpointURLForELBv2(endpointURL string) func(*elasticloadbalancingv2.Options) {
	return func(o *elasticloadbalancingv2.Options) {
		if endpointURL != "" {
			o.BaseEndpoint = aws.String(endpointURL)
		}
	}
}

// addUserAgentMiddleware adds capa-annotator version information to requests made by the AWS SDK.
func addUserAgentMiddleware(stack *middleware.Stack) error {
	return stack.Build.Add(middleware.BuildMiddlewareFunc("CapaAnnotatorUserAgent", func(
		ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler,
	) (
		middleware.BuildOutput, middleware.Metadata, error,
	) {
		// Add custom user agent
		req, ok := in.Request.(*smithyhttp.Request)
		if ok {
			ua := req.Header.Get("User-Agent")
			if ua != "" {
				ua += " "
			}
			req.Header.Set("User-Agent", ua+fmt.Sprintf("github.com/jhjaggars/capa-annotator/%s", version.Version))
		}
		return next.HandleBuild(ctx, in)
	}), middleware.After)
}
