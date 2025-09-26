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

package accelerator_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/rebellions-sw/rbln-k8s-device-plugin/pkg/accelerator"
	"github.com/rebellions-sw/rbln-k8s-device-plugin/pkg/types"
	"github.com/rebellions-sw/rbln-k8s-device-plugin/pkg/types/mocks"
)

var _ = Describe("AccelResourcePool", func() {
	Context("getting a new instance of the pool", func() {
		rc := &types.ResourceConfig{ResourceName: "fake", ResourcePrefix: "fake"}
		pcis := map[string]types.HostDevice{}

		rp := accelerator.NewAccelResourcePool(rc, pcis)

		It("should return a valid instance of the pool", func() {
			Expect(rp).ToNot(BeNil())
		})
	})
	Describe("getting DeviceSpecs", func() {
		Context("for accelerator devices", func() {
			rc := &types.ResourceConfig{
				ResourceName:   "fake",
				ResourcePrefix: "fake",
			}

			// fake1 will have 2 device specs
			fake1 := &mocks.AccelDevice{}
			fake1ds := []*pluginapi.DeviceSpec{
				{ContainerPath: "/fake/path", HostPath: "/dev/fake1a"},
				{ContainerPath: "/fake/path", HostPath: "/dev/fake1b"},
			}
			fake1.On("GetDeviceSpecs").Return(fake1ds)
			fake1.On("GetDriver").Return("fake-driver")

			// fake2 will have 1 device spec
			fake2 := &mocks.AccelDevice{}
			fake2ds := []*pluginapi.DeviceSpec{
				{ContainerPath: "/fake/path", HostPath: "/dev/fake2"},
			}
			fake2.On("GetDeviceSpecs").Return(fake2ds)
			fake2.On("GetDriver").Return("fake-driver")

			// fake3 will have 0 device specs
			fake3 := &mocks.AccelDevice{}
			fake3ds := []*pluginapi.DeviceSpec{}
			fake3.On("GetDeviceSpecs").Return(fake3ds)
			fake3.On("GetDriver").Return("fake-driver")

			pcis := map[string]types.HostDevice{"fake1": fake1, "fake2": fake2, "fake3": fake3}

			rp := accelerator.NewAccelResourcePool(rc, pcis)

			devIDs := []string{"fake1", "fake2"}

			actual := rp.GetDeviceSpecs(devIDs)

			It("should return valid slice of device specs", func() {
				Expect(actual).ToNot(BeNil())
				Expect(actual).To(HaveLen(3)) // fake1 (2) + fake2 (1) => 3 devices (no RSD for fake-driver)
				Expect(actual).To(ContainElement(fake1ds[0]))
				Expect(actual).To(ContainElement(fake1ds[1]))
				Expect(actual).To(ContainElement(fake2ds[0]))
			})
		})
		Context("for rebellions accelerator devices", func() {
			rc := &types.ResourceConfig{
				ResourceName:   "rebellions",
				ResourcePrefix: "rebellions",
			}

			rbln1 := &mocks.AccelDevice{}
			rbln1ds := []*pluginapi.DeviceSpec{
				{ContainerPath: "/dev/rbln", HostPath: "/dev/rbln1"},
			}
			rbln1.On("GetDeviceSpecs").Return(rbln1ds)
			rbln1.On("GetDriver").Return("rebellions")

			rbln2 := &mocks.AccelDevice{}
			rbln2ds := []*pluginapi.DeviceSpec{
				{ContainerPath: "/dev/rbln", HostPath: "/dev/rbln2"},
			}
			rbln2.On("GetDeviceSpecs").Return(rbln2ds)
			rbln2.On("GetDriver").Return("rebellions")

			pcis := map[string]types.HostDevice{"rbln1": rbln1, "rbln2": rbln2}

			rp := accelerator.NewAccelResourcePool(rc, pcis)

			devIDs := []string{"rbln1", "rbln2"}

			actual := rp.GetDeviceSpecs(devIDs)

			It("should return device specs including RSD group device", func() {
				Expect(actual).ToNot(BeNil())
				Expect(actual).To(HaveLen(3)) // rbln1 (1) + rbln2 (1) + rsd group (1) => 3 devices
				Expect(actual).To(ContainElement(rbln1ds[0]))
				Expect(actual).To(ContainElement(rbln2ds[0]))
			})
		})
	})
})
