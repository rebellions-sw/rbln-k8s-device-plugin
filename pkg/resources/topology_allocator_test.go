package resources_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rebellions-sw/rebel-k8s-device-plugin/pkg/resources"
)

var _ = Describe("TopologyAllocator", func() {
	var (
		// Test fixture data based on the provided topology
		availableDeviceIDs  []string
		mockDeviceNUMANodes map[string]int
		mockDeviceBridges   map[string]string
	)

	BeforeEach(func() {
		availableDeviceIDs = []string{
			// NUMA node 0, bridge 0000:02:00.0
			"0000:08:00.0", "0000:07:00.0", "0000:06:00.0", "0000:05:00.0",
			// NUMA node 0, bridge 0000:02:01.0
			"0000:0c:00.0", "0000:0f:00.0", "0000:0e:00.0", "0000:0d:00.0",
			// NUMA node 0, bridge 0000:42:01.0
			"0000:49:00.0", "0000:48:00.0", "0000:47:00.0", "0000:46:00.0",
			// NUMA node 0, bridge 0000:42:02.0
			"0000:4f:00.0", "0000:4e:00.0", "0000:50:00.0", "0000:4d:00.0",
			// NUMA node 1, bridge 0000:c2:00.0
			"0000:c8:00.0", "0000:c7:00.0", "0000:c6:00.0", "0000:c5:00.0",
			// NUMA node 1, bridge 0000:c2:01.0
			"0000:cd:00.0", "0000:cc:00.0", "0000:cf:00.0", "0000:ce:00.0",
			// NUMA node 1, bridge 0000:82:00.0
			"0000:85:00.0", "0000:88:00.0", "0000:87:00.0", "0000:86:00.0",
			// NUMA node 1, bridge 0000:82:01.0
			"0000:8e:00.0", "0000:8d:00.0", "0000:8c:00.0", "0000:8f:00.0",
		}

		mockDeviceNUMANodes = map[string]int{
			// NUMA node 0 devices
			"0000:08:00.0": 0, "0000:07:00.0": 0, "0000:06:00.0": 0, "0000:05:00.0": 0,
			"0000:0c:00.0": 0, "0000:0f:00.0": 0, "0000:0e:00.0": 0, "0000:0d:00.0": 0,
			"0000:49:00.0": 0, "0000:48:00.0": 0, "0000:47:00.0": 0, "0000:46:00.0": 0,
			"0000:4f:00.0": 0, "0000:4e:00.0": 0, "0000:50:00.0": 0, "0000:4d:00.0": 0,
			// NUMA node 1 devices
			"0000:c8:00.0": 1, "0000:c7:00.0": 1, "0000:c6:00.0": 1, "0000:c5:00.0": 1,
			"0000:cd:00.0": 1, "0000:cc:00.0": 1, "0000:cf:00.0": 1, "0000:ce:00.0": 1,
			"0000:85:00.0": 1, "0000:88:00.0": 1, "0000:87:00.0": 1, "0000:86:00.0": 1,
			"0000:8e:00.0": 1, "0000:8d:00.0": 1, "0000:8c:00.0": 1, "0000:8f:00.0": 1,
		}

		mockDeviceBridges = map[string]string{
			// NUMA node 0, bridge 0000:02:00.0
			"0000:08:00.0": "0000:02:00.0", "0000:07:00.0": "0000:02:00.0",
			"0000:06:00.0": "0000:02:00.0", "0000:05:00.0": "0000:02:00.0",
			// NUMA node 0, bridge 0000:02:01.0
			"0000:0c:00.0": "0000:02:01.0", "0000:0f:00.0": "0000:02:01.0",
			"0000:0e:00.0": "0000:02:01.0", "0000:0d:00.0": "0000:02:01.0",
			// NUMA node 0, bridge 0000:42:01.0
			"0000:49:00.0": "0000:42:01.0", "0000:48:00.0": "0000:42:01.0",
			"0000:47:00.0": "0000:42:01.0", "0000:46:00.0": "0000:42:01.0",
			// NUMA node 0, bridge 0000:42:02.0
			"0000:4f:00.0": "0000:42:02.0", "0000:4e:00.0": "0000:42:02.0",
			"0000:50:00.0": "0000:42:02.0", "0000:4d:00.0": "0000:42:02.0",
			// NUMA node 1, bridge 0000:c2:00.0
			"0000:c8:00.0": "0000:c2:00.0", "0000:c7:00.0": "0000:c2:00.0",
			"0000:c6:00.0": "0000:c2:00.0", "0000:c5:00.0": "0000:c2:00.0",
			// NUMA node 1, bridge 0000:c2:01.0
			"0000:cd:00.0": "0000:c2:01.0", "0000:cc:00.0": "0000:c2:01.0",
			"0000:cf:00.0": "0000:c2:01.0", "0000:ce:00.0": "0000:c2:01.0",
			// NUMA node 1, bridge 0000:82:00.0
			"0000:85:00.0": "0000:82:00.0", "0000:88:00.0": "0000:82:00.0",
			"0000:87:00.0": "0000:82:00.0", "0000:86:00.0": "0000:82:00.0",
			// NUMA node 1, bridge 0000:82:01.0
			"0000:8e:00.0": "0000:82:01.0", "0000:8d:00.0": "0000:82:01.0",
			"0000:8c:00.0": "0000:82:01.0", "0000:8f:00.0": "0000:82:01.0",
		}
	})

	// Create a test topology allocator with mocked data
	createTestTopologyAllocator := func(deviceIDs []string) *resources.TopologyAllocator {
		topology := &resources.NodeTopology{
			NUMANodes: make(map[resources.NUMANode]*resources.NUMATopology),
		}

		for _, deviceID := range deviceIDs {
			numaNode := mockDeviceNUMANodes[deviceID]
			pciBridge := mockDeviceBridges[deviceID]

			if _, exists := topology.NUMANodes[resources.NUMANode(numaNode)]; !exists {
				topology.NUMANodes[resources.NUMANode(numaNode)] = &resources.NUMATopology{
					PCIBridges: make(map[resources.BridgeID]*resources.PCIBridgeGroup),
				}
			}

			if _, exists := topology.NUMANodes[resources.NUMANode(numaNode)].PCIBridges[resources.BridgeID(pciBridge)]; !exists {
				topology.NUMANodes[resources.NUMANode(numaNode)].PCIBridges[resources.BridgeID(pciBridge)] = &resources.PCIBridgeGroup{
					DeviceIDs: make([]string, 0),
				}
			}

			topology.NUMANodes[resources.NUMANode(numaNode)].PCIBridges[resources.BridgeID(pciBridge)].DeviceIDs = append(
				topology.NUMANodes[resources.NUMANode(numaNode)].PCIBridges[resources.BridgeID(pciBridge)].DeviceIDs,
				deviceID,
			)
		}

		return &resources.TopologyAllocator{NodeTopology: topology}
	}

	Describe("SelectDevices", func() {
		Context("when devices are available on the same PCI Bridge", func() {
			It("should select devices from the same bridge for allocation size 3", func() {
				allocator := createTestTopologyAllocator(availableDeviceIDs)

				result, err := allocator.SelectDevices([]string{}, 3)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(3))

				// Verify all selected devices are from the same bridge
				// Since we have 4 devices per bridge, allocation size 3 should work
				firstDeviceBridge := mockDeviceBridges[result[0]]
				for _, deviceID := range result {
					Expect(mockDeviceBridges[deviceID]).To(Equal(firstDeviceBridge))
				}
			})

			It("should select devices from the same bridge for allocation size equal to bridge capacity", func() {
				allocator := createTestTopologyAllocator(availableDeviceIDs)

				result, err := allocator.SelectDevices([]string{}, 4)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(4))

				// Verify all selected devices are from the same bridge
				firstDeviceBridge := mockDeviceBridges[result[0]]
				for _, deviceID := range result {
					Expect(mockDeviceBridges[deviceID]).To(Equal(firstDeviceBridge))
				}
			})
		})

		Context("when devices are available on the same NUMA node but not on single bridge", func() {
			It("should select devices from the same NUMA node for allocation size 10", func() {
				allocator := createTestTopologyAllocator(availableDeviceIDs)

				result, err := allocator.SelectDevices([]string{}, 10)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(10))

				// Verify all selected devices are from the same NUMA node
				// Since we have 16 devices per NUMA node, allocation size 10 should work
				firstDeviceNUMA := mockDeviceNUMANodes[result[0]]
				for _, deviceID := range result {
					Expect(mockDeviceNUMANodes[deviceID]).To(Equal(firstDeviceNUMA))
				}
			})

			It("should select devices from the same NUMA node for allocation size equal to NUMA node capacity", func() {
				allocator := createTestTopologyAllocator(availableDeviceIDs)

				result, err := allocator.SelectDevices([]string{}, 16)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(16))

				// Verify all selected devices are from the same NUMA node
				firstDeviceNUMA := mockDeviceNUMANodes[result[0]]
				for _, deviceID := range result {
					Expect(mockDeviceNUMANodes[deviceID]).To(Equal(firstDeviceNUMA))
				}
			})
		})

		Context("when insufficient devices are available for topology-aware allocation", func() {
			It("should return error for allocation size 20", func() {
				allocator := createTestTopologyAllocator(availableDeviceIDs)

				result, err := allocator.SelectDevices([]string{}, 20)

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
				Expect(err.Error()).To(ContainSubstring("insufficient devices for topology-aware allocation"))
			})
		})
	})
})
