// go:build localstack
//go:build localstack

package controller

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	awsclient "github.com/jhjaggars/capa-annotator/pkg/client"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestLocalStackIntegration tests the controller against a running LocalStack instance
// This test requires LocalStack to be running at http://localhost:4566
//
// To run these tests:
//   make localstack-up
//   make test-localstack
//   make localstack-down
func TestLocalStackIntegration(t *testing.T) {
	g := NewWithT(t)

	// Set up LocalStack environment
	os.Setenv("AWS_ENDPOINT_URL", "http://localhost:4566")
	os.Setenv("AWS_ACCESS_KEY_ID", "test")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	os.Setenv("AWS_REGION", "us-east-1")
	defer func() {
		os.Unsetenv("AWS_ENDPOINT_URL")
		os.Unsetenv("AWS_ACCESS_KEY_ID")
		os.Unsetenv("AWS_SECRET_ACCESS_KEY")
		os.Unsetenv("AWS_REGION")
	}()

	// Verify LocalStack is accessible
	ctx := context.Background()
	regionCache := awsclient.NewRegionCache()

	// Try to create a client to verify LocalStack connectivity
	testClient, err := awsclient.NewValidatedClient(nil, "", "", "us-east-1", regionCache)
	if err != nil {
		t.Skipf("LocalStack not available at http://localhost:4566, skipping test: %v", err)
		return
	}
	g.Expect(testClient).ToNot(BeNil())

	t.Run("DescribeInstanceTypes against LocalStack", func(tt *testing.T) {
		gg := NewWithT(tt)

		// Test querying instance types from LocalStack
		client, err := awsclient.NewValidatedClient(nil, "", "", "us-east-1", regionCache)
		gg.Expect(err).ToNot(HaveOccurred())

		// Query for common instance types that LocalStack supports
		// Note: LocalStack community edition has a different set than AWS
		instanceTypes := []types.InstanceType{
			types.InstanceTypeA12xlarge,
			types.InstanceTypeM6g4xlarge,
		}

		cache := NewInstanceTypesCache()
		for _, instanceType := range instanceTypes {
			info, err := cache.GetInstanceType(client, "us-east-1", string(instanceType))
			gg.Expect(err).ToNot(HaveOccurred(), "Failed to get instance type %s", instanceType)
			gg.Expect(info.VCPU).To(BeNumerically(">", 0))
			gg.Expect(info.MemoryMb).To(BeNumerically(">", 0))

			t.Logf("LocalStack returned instance type %s: vCPU=%d, Memory=%dMB, GPU=%d, Arch=%s",
				instanceType, info.VCPU, info.MemoryMb, info.GPU, info.CPUArchitecture)
		}
	})

	t.Run("Cache behavior with LocalStack", func(tt *testing.T) {
		gg := NewWithT(tt)

		client, err := awsclient.NewValidatedClient(nil, "", "", "us-east-1", regionCache)
		gg.Expect(err).ToNot(HaveOccurred())

		cache := NewInstanceTypesCache()
		instanceType := "a1.2xlarge"

		// First call - should hit LocalStack
		start := time.Now()
		info1, err := cache.GetInstanceType(client, "us-east-1", instanceType)
		firstCallDuration := time.Since(start)
		gg.Expect(err).ToNot(HaveOccurred())

		// Second call - should use cache
		start = time.Now()
		info2, err := cache.GetInstanceType(client, "us-east-1", instanceType)
		cachedCallDuration := time.Since(start)
		gg.Expect(err).ToNot(HaveOccurred())

		// Verify cache hit by comparing results and timing
		gg.Expect(info1).To(Equal(info2), "Cached result should match first result")
		gg.Expect(cachedCallDuration).To(BeNumerically("<", firstCallDuration),
			"Cached call should be faster than API call")

		t.Logf("First call: %v, Cached call: %v (speedup: %.2fx)",
			firstCallDuration, cachedCallDuration, float64(firstCallDuration)/float64(cachedCallDuration))
	})

	t.Run("Region validation with LocalStack", func(tt *testing.T) {
		gg := NewWithT(tt)

		// Test valid region
		validClient, err := awsclient.NewValidatedClient(nil, "", "", "us-east-1", regionCache)
		gg.Expect(err).ToNot(HaveOccurred())
		gg.Expect(validClient).ToNot(BeNil())

		// Test with another valid region
		validClient2, err := awsclient.NewValidatedClient(nil, "", "", "us-west-2", regionCache)
		gg.Expect(err).ToNot(HaveOccurred())
		gg.Expect(validClient2).ToNot(BeNil())
	})

	t.Run("Full reconciliation against LocalStack", func(tt *testing.T) {
		gg := NewWithT(tt)

		// Create test CAPI resources
		machineDeployment, awsMachineTemplate, cluster, awsCluster, err := newTestMachineDeployment(
			"default",
			"a1.2xlarge",
			map[string]string{},
		)
		gg.Expect(err).ToNot(HaveOccurred())

		// Create a scheme with CAPI types
		testScheme := runtime.NewScheme()
		gg.Expect(scheme.AddToScheme(testScheme)).To(Succeed())
		gg.Expect(clusterv1.AddToScheme(testScheme)).To(Succeed())
		gg.Expect(infrav1.AddToScheme(testScheme)).To(Succeed())

		// Create fake Kubernetes client with test resources
		fakeK8sClient := fake.NewClientBuilder().
			WithScheme(testScheme).
			WithObjects(machineDeployment, awsMachineTemplate, cluster, awsCluster).
			Build()

		// Use real AWS client builder pointing to LocalStack
		awsClientBuilder := func(ctrlClient client.Client, secretName, namespace, region string, regionCache awsclient.RegionCache) (awsclient.Client, error) {
			return awsclient.NewValidatedClient(ctrlClient, secretName, namespace, region, regionCache)
		}

		r := Reconciler{
			Client:             fakeK8sClient,
			recorder:           record.NewFakeRecorder(10),
			AwsClientBuilder:   awsClientBuilder,
			InstanceTypesCache: NewInstanceTypesCache(),
			RegionCache:        awsclient.NewRegionCache(),
		}

		// Run reconciliation
		result, err := r.reconcile(ctx, machineDeployment)
		gg.Expect(err).ToNot(HaveOccurred())
		gg.Expect(result).ToNot(BeNil())

		// Verify annotations were set
		annotations := machineDeployment.GetAnnotations()
		gg.Expect(annotations).To(HaveKey(cpuKey))
		gg.Expect(annotations).To(HaveKey(memoryKey))
		gg.Expect(annotations).To(HaveKey(gpuKey))
		gg.Expect(annotations).To(HaveKey(labelsKey))

		t.Logf("Reconciliation succeeded with annotations: vCPU=%s, Memory=%sMB, GPU=%s, Arch=%s",
			annotations[cpuKey], annotations[memoryKey], annotations[gpuKey], annotations[labelsKey])
	})
}

// TestLocalStackMultiRegion tests behavior across multiple AWS regions
func TestLocalStackMultiRegion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping multi-region test in short mode")
	}

	// Set up LocalStack environment
	os.Setenv("AWS_ENDPOINT_URL", "http://localhost:4566")
	os.Setenv("AWS_ACCESS_KEY_ID", "test")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	defer func() {
		os.Unsetenv("AWS_ENDPOINT_URL")
		os.Unsetenv("AWS_ACCESS_KEY_ID")
		os.Unsetenv("AWS_SECRET_ACCESS_KEY")
	}()

	regions := []string{"us-east-1", "us-west-2", "eu-west-1"}

	for _, region := range regions {
		t.Run("Region_"+region, func(tt *testing.T) {
			gg := NewWithT(tt)

			regionCache := awsclient.NewRegionCache()
			awsClient, err := awsclient.NewValidatedClient(nil, "", "", region, regionCache)

			// LocalStack may not fully support all regions - skip if not available
			if err != nil {
				tt.Skipf("Region %s not available in LocalStack: %v", region, err)
				return
			}

			gg.Expect(awsClient).ToNot(BeNil())

			// Verify we can query instance types in this region
			cache := NewInstanceTypesCache()
			_, err = cache.GetInstanceType(awsClient, region, "a1.2xlarge")
			gg.Expect(err).ToNot(HaveOccurred())

			tt.Logf("Successfully queried instance type in region %s", region)
		})
	}
}

// TestLocalStackErrorScenarios tests error handling with LocalStack
func TestLocalStackErrorScenarios(t *testing.T) {
	os.Setenv("AWS_ENDPOINT_URL", "http://localhost:4566")
	os.Setenv("AWS_ACCESS_KEY_ID", "test")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	os.Setenv("AWS_REGION", "us-east-1")
	defer func() {
		os.Unsetenv("AWS_ENDPOINT_URL")
		os.Unsetenv("AWS_ACCESS_KEY_ID")
		os.Unsetenv("AWS_SECRET_ACCESS_KEY")
		os.Unsetenv("AWS_REGION")
	}()

	t.Run("Invalid instance type", func(tt *testing.T) {
		gg := NewWithT(tt)

		regionCache := awsclient.NewRegionCache()
		awsClient, err := awsclient.NewValidatedClient(nil, "", "", "us-east-1", regionCache)
		if err != nil {
			tt.Skipf("LocalStack not available: %v", err)
			return
		}

		cache := NewInstanceTypesCache()

		// Query for a non-existent instance type
		_, err = cache.GetInstanceType(awsClient, "us-east-1", "invalid.xlarge")

		// LocalStack may return empty result or error
		// We expect an error for invalid instance types
		gg.Expect(err).To(HaveOccurred(), "Invalid instance type should return an error")

		tt.Logf("Invalid instance type handled gracefully: err=%v", err)
	})

	t.Run("Connection to wrong endpoint", func(tt *testing.T) {
		gg := NewWithT(tt)

		// Point to a non-existent endpoint
		os.Setenv("AWS_ENDPOINT_URL", "http://localhost:9999")
		defer os.Setenv("AWS_ENDPOINT_URL", "http://localhost:4566")

		regionCache := awsclient.NewRegionCache()
		_, err := awsclient.NewValidatedClient(nil, "", "", "us-east-1", regionCache)

		// Should fail to connect
		gg.Expect(err).To(HaveOccurred())
		tt.Logf("Expected connection error: %v", err)
	})
}
