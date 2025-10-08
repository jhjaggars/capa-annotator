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
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	awsclient "github.com/jhjaggars/capa-annotator/pkg/client"
	"k8s.io/klog/v2"
)

// we define this additional type as the EC2 API returns the architecture in a different format than the one we use.
type normalizedArch string

const (
	// ArchitectureAmd64 is the normalized architecture name for amd64.
	ArchitectureAmd64 normalizedArch = "amd64"
	// ArchitectureArm64 is the normalized architecture name for arm64.
	ArchitectureArm64 normalizedArch = "arm64"
)

// InstanceType holds some of the instance type information that we need to store.
type InstanceType struct {
	InstanceType    string
	VCPU            int64
	MemoryMb        int64
	GPU             int64
	CPUArchitecture normalizedArch
}

// InstanceTypesCache is a cache for instance type information.
type InstanceTypesCache interface {
	GetInstanceType(awsClient awsclient.Client, cacheID string, instanceType string) (InstanceType, error)
}

// instanceTypesRegion holds cached instance types for specific region and time when it was last updated.
type instanceTypesRegion struct {
	instanceTypes map[string]InstanceType
	lastUpdate    time.Time
}

// instanceTypesCache holds cached instance types per region. Acess is synchronized via rwmutex.
type instanceTypesCache struct {
	cache   map[string]instanceTypesRegion
	rwmutex sync.RWMutex
}

// NewInstanceTypesCache creates an empty instance types cache.
func NewInstanceTypesCache() InstanceTypesCache {
	cache := &instanceTypesCache{}
	cache.cache = map[string]instanceTypesRegion{}
	cache.rwmutex = sync.RWMutex{}
	return cache
}

// GetInstanceType retrievees InstanceType from cache by name. If the cache is stale or nil it is refreshed first from the EC2 API.
// The fetched instance types are specific to the region of the awsClient. Using region name as cacheID is recomended.
func (i *instanceTypesCache) GetInstanceType(awsClient awsclient.Client, cacheID string, instanceType string) (InstanceType, error) {
	i.rwmutex.RLock()

	if !i.isCacheFresh(cacheID) {
		i.rwmutex.RUnlock()
		if err := i.refresh(awsClient, cacheID); err != nil {
			return InstanceType{}, fmt.Errorf("error refreshing instance types cache: %w", err)
		}
		i.rwmutex.RLock()
	}

	instanceTypeInfo, ok := i.cache[cacheID].instanceTypes[instanceType]
	if !ok {
		instanceNames := []string{}
		for _, instanceType := range i.cache[cacheID].instanceTypes {
			instanceNames = append(instanceNames, instanceType.InstanceType)
		}
		i.rwmutex.RUnlock()
		return InstanceType{}, fmt.Errorf("instance type %q not found: The valid instance types in the current region are: %q", instanceType, instanceNames)
	}

	i.rwmutex.RUnlock()
	return instanceTypeInfo, nil
}

// isCacheFresh checks whether the cache for given cacheId is populated and has been refreshed in the last 24 hours.
func (i *instanceTypesCache) isCacheFresh(cacheID string) bool {
	cacheForRegion, ok := i.cache[cacheID]
	return ok && cacheForRegion.instanceTypes != nil && cacheForRegion.lastUpdate.After(time.Now().Add(-24*time.Hour))
}

// refresh ensures that the cache is updated in a thread safe way.
func (i *instanceTypesCache) refresh(awsClient awsclient.Client, cacheID string) error {
	// Only one thread should refresh the cache at a time.
	// Parallel refresh does not speed up the process and can cause throttling.
	i.rwmutex.Lock()
	defer i.rwmutex.Unlock()

	if i.isCacheFresh(cacheID) {
		// Another thread has already refreshed the cache.
		return nil
	}

	instanceTypes, err := fetchEC2InstanceTypes(awsClient)
	if err != nil {
		return fmt.Errorf("failed to refresh instance types cache: %w", err)
	}

	i.cache[cacheID] = instanceTypesRegion{instanceTypes: instanceTypes, lastUpdate: time.Now()}
	return nil
}

// fetchEC2InstanceTypes fetches all available instance types from EC2 API.
func fetchEC2InstanceTypes(awsClient awsclient.Client) (map[string]InstanceType, error) {
	klog.V(3).Info("Refreshing instance types cache")

	if awsClient == nil {
		return nil, errors.New("awsClient is nil")
	}

	input := ec2.DescribeInstanceTypesInput{}
	instanceTypes := make(map[string]InstanceType)

	// AWS API paginates responses, so we need to loop until we get all the results
	requestCounter := 0
	for {
		requestCounter++
		rawInstanceTypes, err := awsClient.DescribeInstanceTypes(&input)
		if err != nil {
			return nil, fmt.Errorf("describeInstanceTypes request failed: %w", err)
		}
		for _, rawInstanceType := range rawInstanceTypes.InstanceTypes {
			if rawInstanceType.InstanceType == nil && *rawInstanceType.InstanceType != "" {
				return nil, fmt.Errorf("describeInstanceTypes returned instance type with nil or empty instance name")
			}
			instanceTypes[*rawInstanceType.InstanceType] = transformInstanceType(rawInstanceType)
		}

		// If next token is empty, we have all the results
		if rawInstanceTypes.NextToken == nil {
			break
		}
		input.NextToken = rawInstanceTypes.NextToken
	}

	if len(instanceTypes) == 0 {
		return nil, errors.New("unable to load EC2 Instance Type list")
	}

	klog.V(4).Infof("Fetched instance types data in %d requests", requestCounter)
	return instanceTypes, nil
}

// transformInstanceType takes information we care about from ec2.InstanceTypeInfo and transforms it into InstanceType.
func transformInstanceType(rawInstanceType *ec2.InstanceTypeInfo) InstanceType {
	instanceType := InstanceType{
		InstanceType: *rawInstanceType.InstanceType,
	}
	if rawInstanceType.MemoryInfo != nil && rawInstanceType.MemoryInfo.SizeInMiB != nil {
		instanceType.MemoryMb = *rawInstanceType.MemoryInfo.SizeInMiB
	}
	if rawInstanceType.VCpuInfo != nil && rawInstanceType.VCpuInfo.DefaultVCpus != nil {
		instanceType.VCPU = *rawInstanceType.VCpuInfo.DefaultVCpus
	}
	if rawInstanceType.GpuInfo != nil && len(rawInstanceType.GpuInfo.Gpus) > 0 {
		instanceType.GPU = getGpuCount(rawInstanceType.GpuInfo)
	}
	if rawInstanceType.ProcessorInfo != nil && len(rawInstanceType.ProcessorInfo.SupportedArchitectures) > 0 &&
		rawInstanceType.ProcessorInfo.SupportedArchitectures[0] != nil && *rawInstanceType.ProcessorInfo.SupportedArchitectures[0] != "" {
		instanceType.CPUArchitecture = normalizeArchitecture(*rawInstanceType.ProcessorInfo.SupportedArchitectures[0])
	} else {
		instanceType.CPUArchitecture = normalizeArchitecture("amd64")
	}
	return instanceType
}

// getGpuCount counts all the GPUs in GpuInfo.
func getGpuCount(gpuInfo *ec2.GpuInfo) int64 {
	gpuCountSum := int64(0)
	for _, gpu := range gpuInfo.Gpus {
		if gpu.Count != nil {
			gpuCountSum += *gpu.Count
		}
	}
	return gpuCountSum
}

// normalizeArchitecture converts the given architecture string from the format used by the EC2 API to the one for kubernetes.
// In particular, at the time of writing,
// the EC2 API uses the GNU name for the x86_64 architecture, and the Golang/LLVM name for the aarch64.
// The kubernetes.io/arch label expects the Golang/LLVM names.
// See vendor/github.com/aws/aws-sdk-go/service/ec2/api.go
func normalizeArchitecture(architecture string) normalizedArch {
	switch architecture {
	case ec2.ArchitectureTypeX8664:
		return ArchitectureAmd64
	case ec2.ArchitectureTypeArm64:
		return ArchitectureArm64
	}
	klog.V(2).Infof("unknown architecture %s. Defaulting to amd64", architecture)
	// Default to amd64 if we don't recognize the architecture.
	return ArchitectureAmd64
}
