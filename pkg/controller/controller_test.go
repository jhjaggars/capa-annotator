/*
Copyright The Kubernetes Authors.
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

package controller

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/service/ec2"

	awsclient "github.com/jhjaggars/capa-annotator/pkg/client"
	fakeawsclient "github.com/jhjaggars/capa-annotator/pkg/client/fake"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gtypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var _ = Describe("MachineDeploymentReconciler", func() {
	var c client.Client
	var stopMgr context.CancelFunc
	var fakeRecorder *record.FakeRecorder
	var namespace *corev1.Namespace
	fakeClient, err := fakeawsclient.NewClient(nil, "", "", "")
	Expect(err).ToNot(HaveOccurred())
	awsClientBuilder := func(client client.Client, secretName, namespace, region string, regionCache awsclient.RegionCache) (awsclient.Client, error) {
		return fakeClient, nil
	}

	BeforeEach(func() {
		mgr, err := manager.New(cfg, manager.Options{
			Metrics: server.Options{
				BindAddress: "0",
			}})
		Expect(err).ToNot(HaveOccurred())

		r := Reconciler{
			Client:             mgr.GetClient(),
			Log:                log.Log,
			AwsClientBuilder:   awsClientBuilder,
			InstanceTypesCache: NewInstanceTypesCache(),
		}
		Expect(r.SetupWithManager(mgr, controller.Options{
			SkipNameValidation: ptr.To(true),
		})).To(Succeed())

		fakeRecorder = record.NewFakeRecorder(1)
		r.recorder = fakeRecorder

		c = mgr.GetClient()
		stopMgr = StartTestManager(mgr)

		namespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "mhc-test-"}}
		Expect(c.Create(ctx, namespace)).To(Succeed())
	})

	AfterEach(func() {
		Expect(deleteMachineDeployments(c, namespace.Name)).To(Succeed())
		stopMgr()
	})

	type reconcileTestCase = struct {
		instanceType        string
		existingAnnotations map[string]string
		expectedAnnotations map[string]string
		expectedEvents      []string
	}

	DescribeTable("when reconciling MachineDeployments", func(rtc reconcileTestCase) {
		// Set IRSA env vars for tests
		os.Setenv("AWS_ROLE_ARN", "arn:aws:iam::123456789012:role/test-role")
		os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "/var/run/secrets/eks.amazonaws.com/serviceaccount/token")
		defer os.Unsetenv("AWS_ROLE_ARN")
		defer os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE")

		machineDeployment, awsMachineTemplate, cluster, awsCluster, err := newTestMachineDeployment(namespace.Name, rtc.instanceType, rtc.existingAnnotations)
		Expect(err).ToNot(HaveOccurred())

		// Create infrastructure resources first
		Expect(c.Create(ctx, awsCluster)).To(Succeed())
		Expect(c.Create(ctx, cluster)).To(Succeed())
		Expect(c.Create(ctx, awsMachineTemplate)).To(Succeed())
		Expect(c.Create(ctx, machineDeployment)).To(Succeed())

		Eventually(func() map[string]string {
			md := &clusterv1.MachineDeployment{}
			key := client.ObjectKey{Namespace: machineDeployment.Namespace, Name: machineDeployment.Name}
			err := c.Get(ctx, key, md)
			if err != nil {
				return nil
			}
			annotations := md.GetAnnotations()
			if annotations != nil {
				return annotations
			}
			// Return an empty map to distinguish between empty annotations and errors
			return make(map[string]string)
		}, timeout).Should(Equal(rtc.expectedAnnotations))

		// Check which event types were sent
		Eventually(fakeRecorder.Events, timeout).Should(HaveLen(len(rtc.expectedEvents)))
		receivedEvents := []string{}
		eventMatchers := []gtypes.GomegaMatcher{}
		for _, ev := range rtc.expectedEvents {
			receivedEvents = append(receivedEvents, <-fakeRecorder.Events)
			eventMatchers = append(eventMatchers, ContainSubstring(fmt.Sprintf(" %s ", ev)))
		}
		Expect(receivedEvents).To(ConsistOf(eventMatchers))
	},
	// Skip "with no instanceType set" - CAPA CRDs require instanceType >= 2 chars
	// This scenario is covered by unit tests without CRD validation
// 		Entry("with no instanceType set", reconcileTestCase{
// 			instanceType:        "",
// 			existingAnnotations: make(map[string]string),
// 			expectedAnnotations: make(map[string]string),
// 			expectedEvents:      []string{"FailedUpdate"},
// 		}),
		Entry("with a a1.2xlarge", reconcileTestCase{
			instanceType:        "a1.2xlarge",
			existingAnnotations: make(map[string]string),
			expectedAnnotations: map[string]string{
				cpuKey:    "8",
				memoryKey: "16384",
				gpuKey:    "0",
				labelsKey: "kubernetes.io/arch=amd64",
			},
			expectedEvents: []string{},
		}),
		Entry("with a p2.16xlarge", reconcileTestCase{
			instanceType:        "p2.16xlarge",
			existingAnnotations: make(map[string]string),
			expectedAnnotations: map[string]string{
				cpuKey:    "64",
				memoryKey: "749568",
				gpuKey:    "16",
				labelsKey: "kubernetes.io/arch=amd64",
			},
			expectedEvents: []string{},
		}),
		Entry("with existing annotations", reconcileTestCase{
			instanceType: "a1.2xlarge",
			existingAnnotations: map[string]string{
				"existing": "annotation",
				"annother": "existingAnnotation",
			},
			expectedAnnotations: map[string]string{
				"existing": "annotation",
				"annother": "existingAnnotation",
				cpuKey:     "8",
				memoryKey:  "16384",
				gpuKey:     "0",
				labelsKey:  "kubernetes.io/arch=amd64",
			},
			expectedEvents: []string{},
		}),
		Entry("with a m6g.4xlarge (aarch64)", reconcileTestCase{
			instanceType:        "m6g.4xlarge",
			existingAnnotations: make(map[string]string),
			expectedAnnotations: map[string]string{
				cpuKey:    "16",
				memoryKey: "65536",
				gpuKey:    "0",
				labelsKey: "kubernetes.io/arch=arm64",
			},
			expectedEvents: []string{},
		}),
		Entry("with an instance type missing the supported architecture (default to amd64)", reconcileTestCase{
			instanceType:        "m6i.8xlarge",
			existingAnnotations: make(map[string]string),
			expectedAnnotations: map[string]string{
				cpuKey:    "32",
				memoryKey: "131072",
				gpuKey:    "0",
				labelsKey: "kubernetes.io/arch=amd64",
			},
			expectedEvents: []string{},
		}),
		Entry("with an unrecognized supported architecture (default to amd64)", reconcileTestCase{
			instanceType:        "m6h.8xlarge",
			existingAnnotations: make(map[string]string),
			expectedAnnotations: map[string]string{
				cpuKey:    "32",
				memoryKey: "131072",
				gpuKey:    "0",
				labelsKey: "kubernetes.io/arch=amd64",
			},
			expectedEvents: []string{},
		}),
		Entry("with an invalid instanceType", reconcileTestCase{
			instanceType: "invalid",
			existingAnnotations: map[string]string{
				"existing": "annotation",
				"annother": "existingAnnotation",
			},
			expectedAnnotations: map[string]string{
				"existing": "annotation",
				"annother": "existingAnnotation",
			},
			expectedEvents: []string{"FailedUpdate"},
		}),
	)
})

func deleteMachineDeployments(c client.Client, namespaceName string) error {
	machineDeployments := &clusterv1.MachineDeploymentList{}
	err := c.List(ctx, machineDeployments, client.InNamespace(namespaceName))
	if err != nil {
		return err
	}

	for _, md := range machineDeployments.Items {
		err := c.Delete(ctx, &md)
		if err != nil {
			return err
		}
	}

	Eventually(func() error {
		machineDeployments := &clusterv1.MachineDeploymentList{}
		err := c.List(ctx, machineDeployments)
		if err != nil {
			return err
		}
		if len(machineDeployments.Items) > 0 {
			return fmt.Errorf("machineDeployments not deleted")
		}
		return nil
	}, timeout).Should(Succeed())

	return nil
}

func TestReconcile(t *testing.T) {
	testCases := []struct {
		name                string
		instanceType        string
		existingAnnotations map[string]string
		expectedAnnotations map[string]string
		expectErr           bool
	}{
		{
			name:                "with no instanceType set",
			instanceType:        "",
			existingAnnotations: make(map[string]string),
			expectedAnnotations: make(map[string]string),
			// Expect error when instanceType is empty - cannot determine CPU/memory/GPU
			expectErr: true,
		},
		{
			name:                "with a a1.2xlarge",
			instanceType:        "a1.2xlarge",
			existingAnnotations: make(map[string]string),
			expectedAnnotations: map[string]string{
				cpuKey:    "8",
				memoryKey: "16384",
				gpuKey:    "0",
				labelsKey: "kubernetes.io/arch=amd64",
			},
			expectErr: false,
		},
		{
			name:                "with a p2.16xlarge",
			instanceType:        "p2.16xlarge",
			existingAnnotations: make(map[string]string),
			expectedAnnotations: map[string]string{
				cpuKey:    "64",
				memoryKey: "749568",
				gpuKey:    "16",
				labelsKey: "kubernetes.io/arch=amd64",
			},
			expectErr: false,
		},
		{
			name:         "with existing annotations",
			instanceType: "a1.2xlarge",
			existingAnnotations: map[string]string{
				"existing": "annotation",
				"annother": "existingAnnotation",
			},
			expectedAnnotations: map[string]string{
				"existing": "annotation",
				"annother": "existingAnnotation",
				cpuKey:     "8",
				memoryKey:  "16384",
				gpuKey:     "0",
				labelsKey:  "kubernetes.io/arch=amd64",
			},
			expectErr: false,
		},
		{
			name:         "with an invalid instanceType",
			instanceType: "invalid",
			existingAnnotations: map[string]string{
				"existing": "annotation",
				"annother": "existingAnnotation",
			},
			expectedAnnotations: map[string]string{
				"existing": "annotation",
				"annother": "existingAnnotation",
			},
			// Expect no error for invalid instanceType - logs warning but does not fail reconciliation
			expectErr: false,
		},
		{
			name:                "with a m6g.4xlarge (aarch64)",
			instanceType:        "m6g.4xlarge",
			existingAnnotations: make(map[string]string),
			expectedAnnotations: map[string]string{
				cpuKey:    "16",
				memoryKey: "65536",
				gpuKey:    "0",
				labelsKey: "kubernetes.io/arch=arm64",
			},
			expectErr: false,
		},
		{
			name:                "with an instance type missing the supported architecture (default to amd64)",
			instanceType:        "m6i.8xlarge",
			existingAnnotations: make(map[string]string),
			expectedAnnotations: map[string]string{
				cpuKey:    "32",
				memoryKey: "131072",
				gpuKey:    "0",
				labelsKey: "kubernetes.io/arch=amd64",
			},
			expectErr: false,
		},
		{
			name:                "with an unrecognized supported architecture (default to amd64)",
			instanceType:        "m6h.8xlarge",
			existingAnnotations: make(map[string]string),
			expectedAnnotations: map[string]string{
				cpuKey:    "32",
				memoryKey: "131072",
				gpuKey:    "0",
				labelsKey: "kubernetes.io/arch=amd64",
			},
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			g := NewWithT(tt)

			// Set IRSA environment variables for tests
			os.Setenv("AWS_ROLE_ARN", "arn:aws:iam::123456789012:role/test-role")
			os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "/var/run/secrets/eks.amazonaws.com/serviceaccount/token")
			defer os.Unsetenv("AWS_ROLE_ARN")
			defer os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE")

			// Create test resources
			machineDeployment, awsMachineTemplate, cluster, awsCluster, err := newTestMachineDeployment("default", tc.instanceType, tc.existingAnnotations)
			g.Expect(err).ToNot(HaveOccurred())

			// Create a scheme with CAPI types
			testScheme := runtime.NewScheme()
			g.Expect(scheme.AddToScheme(testScheme)).To(Succeed())
			g.Expect(clusterv1.AddToScheme(testScheme)).To(Succeed())
			g.Expect(infrav1.AddToScheme(testScheme)).To(Succeed())

			// Create fake Kubernetes client with test resources
			fakeK8sClient := fake.NewClientBuilder().
				WithScheme(testScheme).
				WithObjects(machineDeployment, awsMachineTemplate, cluster, awsCluster).
				Build()

			// Create fake AWS client
			fakeAWSClient, err := fakeawsclient.NewClient(nil, "", "", "")
			g.Expect(err).ToNot(HaveOccurred())
			awsClientBuilder := func(client client.Client, secretName, namespace, region string, regionCache awsclient.RegionCache) (awsclient.Client, error) {
				return fakeAWSClient, nil
			}

			r := Reconciler{
				Client:             fakeK8sClient,
				recorder:           record.NewFakeRecorder(1),
				AwsClientBuilder:   awsClientBuilder,
				InstanceTypesCache: NewInstanceTypesCache(),
			}

			_, err = r.reconcile(machineDeployment)
			g.Expect(err != nil).To(Equal(tc.expectErr))
			g.Expect(machineDeployment.Annotations).To(Equal(tc.expectedAnnotations))
		})
	}
}

func TestReconcileWithIRSA(t *testing.T) {
	testCases := []struct {
		name                string
		instanceType        string
		setIRSAEnvVars      bool
		expectErr           bool
		errorContains       string
		expectedAnnotations map[string]string
	}{
		{
			name:           "with IRSA configured",
			instanceType:   "a1.2xlarge",
			setIRSAEnvVars: true,
			expectErr:      false,
			expectedAnnotations: map[string]string{
				cpuKey:    "8",
				memoryKey: "16384",
				gpuKey:    "0",
				labelsKey: "kubernetes.io/arch=amd64",
			},
		},
		{
			name:           "without IRSA configured - falls back to default credential chain",
			instanceType:   "a1.2xlarge",
			setIRSAEnvVars: false,
			expectErr:      false,
			expectedAnnotations: map[string]string{
				cpuKey:    "8",
				memoryKey: "16384",
				gpuKey:    "0",
				labelsKey: "kubernetes.io/arch=amd64",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(tt *testing.T) {
			g := NewWithT(tt)

			// Set up or clear IRSA environment variables
			if tc.setIRSAEnvVars {
				os.Setenv("AWS_ROLE_ARN", "arn:aws:iam::123456789012:role/test-role")
				os.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "/var/run/secrets/eks.amazonaws.com/serviceaccount/token")
				defer os.Unsetenv("AWS_ROLE_ARN")
				defer os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE")
			} else {
				os.Unsetenv("AWS_ROLE_ARN")
				os.Unsetenv("AWS_WEB_IDENTITY_TOKEN_FILE")
			}

			machineDeployment, awsMachineTemplate, cluster, awsCluster, err := newTestMachineDeployment("default", tc.instanceType, make(map[string]string))
			g.Expect(err).ToNot(HaveOccurred())

		// Create a scheme with CAPI types
		testScheme := runtime.NewScheme()
		g.Expect(scheme.AddToScheme(testScheme)).To(Succeed())
		g.Expect(clusterv1.AddToScheme(testScheme)).To(Succeed())
		g.Expect(infrav1.AddToScheme(testScheme)).To(Succeed())

		// Create fake Kubernetes client with test resources
		fakeK8sClient := fake.NewClientBuilder().
			WithScheme(testScheme).
			WithObjects(machineDeployment, awsMachineTemplate, cluster, awsCluster).
			Build()

			fakeAWSClient, err := fakeawsclient.NewClient(nil, "", "", "")
			g.Expect(err).ToNot(HaveOccurred())
			awsClientBuilder := func(client client.Client, secretName, namespace, region string, regionCache awsclient.RegionCache) (awsclient.Client, error) {
				// Mock supports both IRSA and fallback to default credential chain
				return fakeAWSClient, nil
			}

		r := Reconciler{
			Client:             fakeK8sClient,
			recorder:           record.NewFakeRecorder(1),
			AwsClientBuilder:   awsClientBuilder,
			InstanceTypesCache: NewInstanceTypesCache(),
		}
			_, err = r.reconcile(machineDeployment)
			if tc.expectErr {
				g.Expect(err).To(HaveOccurred())
				if tc.errorContains != "" {
					g.Expect(err.Error()).To(ContainSubstring(tc.errorContains))
				}
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(machineDeployment.Annotations).To(Equal(tc.expectedAnnotations))
			}
		})
	}
}

func TestNormalizeArchitecture(t *testing.T) {
	testCases := []struct {
		architecture string
		expected     normalizedArch
	}{
		{
			architecture: ec2.ArchitectureTypeX8664,
			expected:     ArchitectureAmd64,
		},
		{
			architecture: ec2.ArchitectureTypeArm64,
			expected:     ArchitectureArm64,
		},
		{
			architecture: "unknown",
			expected:     ArchitectureAmd64,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.architecture, func(tt *testing.T) {
			g := NewWithT(tt)
			g.Expect(normalizeArchitecture(tc.architecture)).To(Equal(tc.expected))
		})
	}
}

// newTestMachineDeployment creates a test CAPI MachineDeployment with supporting infrastructure
func newTestMachineDeployment(namespace, instanceType string, existingAnnotations map[string]string) (*clusterv1.MachineDeployment, *infrav1.AWSMachineTemplate, *clusterv1.Cluster, *infrav1.AWSCluster, error) {
	annotations := make(map[string]string)
	for k, v := range existingAnnotations {
		annotations[k] = v
	}

	// Create AWSMachineTemplate
	awsMachineTemplate := &infrav1.AWSMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-aws-template",
			Namespace: namespace,
		},
		Spec: infrav1.AWSMachineTemplateSpec{
			Template: infrav1.AWSMachineTemplateResource{
				Spec: infrav1.AWSMachineSpec{
					InstanceType: instanceType,
				},
			},
		},
	}

	// Create AWSCluster
	awsCluster := &infrav1.AWSCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster-aws",
			Namespace: namespace,
		},
		Spec: infrav1.AWSClusterSpec{
			Region: "us-east-1",
		},
	}

	// Create Cluster
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: namespace,
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: &corev1.ObjectReference{
				APIVersion: "infrastructure.cluster.x-k8s.io/v1beta2",
				Kind:       "AWSCluster",
				Name:       awsCluster.Name,
				Namespace:  awsCluster.Namespace,
			},
		},
	}

	// Create MachineDeployment
	replicas := int32(1)
	machineDeployment := &clusterv1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Annotations:  annotations,
			GenerateName: "test-md-",
			Namespace:    namespace,
		},
		Spec: clusterv1.MachineDeploymentSpec{
			ClusterName: cluster.Name,
			Replicas:    &replicas,
			Template: clusterv1.MachineTemplateSpec{
				Spec: clusterv1.MachineSpec{
					ClusterName: cluster.Name,
					InfrastructureRef: corev1.ObjectReference{
						APIVersion: "infrastructure.cluster.x-k8s.io/v1beta2",
						Kind:       "AWSMachineTemplate",
						Name:       awsMachineTemplate.Name,
						Namespace:  awsMachineTemplate.Namespace,
					},
				},
			},
		},
	}

	return machineDeployment, awsMachineTemplate, cluster, awsCluster, nil
}
