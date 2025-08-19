// Copyright 2020 Intel Corp. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package accelerator

import (
	"github.com/jaypipes/ghw"

	"github.com/rebellions-sw/rebel-k8s-device-plugin/pkg/devices"
	"github.com/rebellions-sw/rebel-k8s-device-plugin/pkg/infoprovider"
	"github.com/rebellions-sw/rebel-k8s-device-plugin/pkg/types"
	"github.com/rebellions-sw/rebel-k8s-device-plugin/pkg/utils"
)

// accelDevice extends HostDevice and embedds GenPciDevice
type accelDevice struct {
	types.HostDevice
	devices.GenPciDevice
}

// NewAccelDevice returns an instance of AccelDevice interface
func NewAccelDevice(dev *ghw.PCIDevice, rFactory types.ResourceFactory,
	rc *types.ResourceConfig) (types.AccelDevice, error) {
	infoProviders := make([]types.DeviceInfoProvider, 0)

	driverName, err := utils.GetDriverName(dev.Address)
	if err != nil {
		return nil, err
	}

	// rebellions: add rebellions info provider
	if dev.Vendor.ID == infoprovider.RebellionsVendorID {
		if driverName == "rebellions" {
			infoProviders = append(infoProviders, infoprovider.NewRebellionsInfoProvider(dev.Address))
		} else if driverName == "vfio-pci" {
			infoProviders = append(infoProviders, infoprovider.NewVfioInfoProvider(dev.Address))
		}
	}

	hostDev, err := devices.NewHostDeviceImpl(dev, dev.Address, rFactory, rc, infoProviders)
	if err != nil {
		return nil, err
	}
	pciDev, err := devices.NewGenPciDevice(dev)
	if err != nil {
		return nil, err
	}

	return &accelDevice{
		HostDevice:   hostDev,
		GenPciDevice: *pciDev,
	}, nil
}
