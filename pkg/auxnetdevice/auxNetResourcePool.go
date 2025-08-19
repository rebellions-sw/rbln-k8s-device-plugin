/*
 * SPDX-FileCopyrightText: Copyright (c) 2022 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package auxnetdevice

import (
	"github.com/golang/glog"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/rebellions-sw/rbln-k8s-device-plugin/pkg/resources"
	"github.com/rebellions-sw/rbln-k8s-device-plugin/pkg/types"
)

const (
	auxPoolType = "net-sf"
)

type auxNetResourcePool struct {
	*resources.ResourcePoolImpl
}

var _ types.ResourcePool = &auxNetResourcePool{}

// NewAuxNetResourcePool returns an instance of resourcePool
func NewAuxNetResourcePool(rc *types.ResourceConfig, devicePool map[string]types.HostDevice) types.ResourcePool {
	rp := resources.NewResourcePool(rc, devicePool)
	return &auxNetResourcePool{
		ResourcePoolImpl: rp,
	}
}

// Overrides GetDeviceSpecs
func (ap *auxNetResourcePool) GetDeviceSpecs(deviceIDs []string) []*pluginapi.DeviceSpec {
	glog.Infof("GetDeviceSpecs(): for devices: %v", deviceIDs)
	devSpecs := make([]*pluginapi.DeviceSpec, 0)

	devicePool := ap.GetDevicePool()

	// Add device driver specific and rdma specific devices
	for _, id := range deviceIDs {
		if dev, ok := devicePool[id]; ok {
			auxDev := dev.(types.AuxNetDevice) // convert generic HostDevice to AuxNetDevice
			newSpecs := auxDev.GetDeviceSpecs()
			for _, ds := range newSpecs {
				if !ap.DeviceSpecExist(devSpecs, ds) {
					devSpecs = append(devSpecs, ds)
				}
			}
		}
	}
	return devSpecs
}

// GetCDIKind returns device kind for CDI spec
func (ap *auxNetResourcePool) GetCDIName() string {
	return auxPoolType
}
