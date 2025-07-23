package resources

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TopologyAllocator", func() {

	// Helper function to create test topology with specified bridge capacities
	createTestTopologyWithBridges := func(bridgeCapacities []int) *TopologyAllocator {
		topology := &NodeTopology{
			NUMANodes:            make(map[NUMANode]*NUMATopology),
			PciBridgeDeviceCount: 10,  // Default bridge capacity
			NumaNodeDeviceCount:  100, // Large enough to not interfere with tests
		}

		numaTopology := &NUMATopology{
			PCIBridges: make(map[BridgeID]*PCIBridgeGroup),
		}

		deviceCounter := 0
		for bridgeIdx, capacity := range bridgeCapacities {
			bridgeID := BridgeID(fmt.Sprintf("bridge%d", bridgeIdx))
			deviceIDs := make([]string, capacity)

			for i := 0; i < capacity; i++ {
				deviceIDs[i] = fmt.Sprintf("device%d", deviceCounter)
				deviceCounter++
			}

			numaTopology.PCIBridges[bridgeID] = &PCIBridgeGroup{
				DeviceIDs: deviceIDs,
			}
		}

		topology.NUMANodes[NUMANode(0)] = numaTopology

		return &TopologyAllocator{NodeTopology: topology}
	}

	// Helper function to get bridge assignment of selected devices
	getBridgeAssignment := func(allocator *TopologyAllocator, selectedDevices []string) map[string]int {
		bridgeAssignment := make(map[string]int)

		for numaIdx, numa := range allocator.NodeTopology.NUMANodes {
			for bridgeID, bridge := range numa.PCIBridges {
				for _, deviceID := range bridge.DeviceIDs {
					for _, selectedDevice := range selectedDevices {
						if deviceID == selectedDevice {
							bridgeKey := fmt.Sprintf("numa%d_bridge%s", numaIdx, bridgeID)
							bridgeAssignment[bridgeKey]++
						}
					}
				}
			}
		}
		return bridgeAssignment
	}

	Describe("Bridge Minimization", func() {
		Context("when allocation can be satisfied with fewer bridges", func() {
			It("should prefer single bridge over multiple bridges for allocation size 4", func() {
				// Bridge capacities: [4, 3, 2, 1] - should select bridge0 (4 devices)
				allocator := createTestTopologyWithBridges([]int{4, 3, 2, 1})

				result, err := allocator.SelectDevices([]string{}, 4)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(4))

				bridgeAssignment := getBridgeAssignment(allocator, result)
				Expect(len(bridgeAssignment)).To(Equal(1), "Should use only 1 bridge")

				// Should use bridge0 which has exactly 4 devices
				Expect(bridgeAssignment["numa0_bridgebridge0"]).To(Equal(4))
			})

			It("should prefer single bridge over multiple bridges for allocation size 3", func() {
				// Bridge capacities: [4, 3, 2, 1] - should select bridge1
				allocator := createTestTopologyWithBridges([]int{4, 3, 2, 1})

				result, err := allocator.SelectDevices([]string{}, 3)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(3))

				bridgeAssignment := getBridgeAssignment(allocator, result)
				Expect(len(bridgeAssignment)).To(Equal(1), "Should use only 1 bridge")

				Expect(bridgeAssignment["numa0_bridgebridge1"]).To(Equal(3))
			})
		})

		Context("when multiple bridges are required", func() {
			It("should use minimum bridges for allocation size 6", func() {
				// Bridge capacities: [4, 3, 2, 1] - should use 2 bridges: bridge0(4) + bridge2(2)
				allocator := createTestTopologyWithBridges([]int{4, 3, 2, 1})

				result, err := allocator.SelectDevices([]string{}, 6)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(6))

				bridgeAssignment := getBridgeAssignment(allocator, result)
				Expect(len(bridgeAssignment)).To(Equal(2), "Should use exactly 2 bridges")

				Expect(bridgeAssignment["numa0_bridgebridge0"]).To(Equal(4))
				Expect(bridgeAssignment["numa0_bridgebridge2"]).To(Equal(2))
			})

			It("should use optimal multiple bridge combination for allocation size 8", func() {
				// Bridge capacities: [4, 3, 2, 1] - should use 3 bridges for size 8
				allocator := createTestTopologyWithBridges([]int{4, 3, 2, 1})

				result, err := allocator.SelectDevices([]string{}, 8)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(8))

				bridgeAssignment := getBridgeAssignment(allocator, result)
				Expect(len(bridgeAssignment)).To(Equal(3), "Should use exactly 3 bridges")

				// Verify allocation counts
				Expect(bridgeAssignment["numa0_bridgebridge0"]).To(Equal(4))
				Expect(bridgeAssignment["numa0_bridgebridge1"]).To(Equal(3))
				Expect(bridgeAssignment["numa0_bridgebridge3"]).To(Equal(1))
			})

			It("should handle allocation size requiring all bridges", func() {
				// Bridge capacities: [3, 2, 1] - for allocation size 6
				allocator := createTestTopologyWithBridges([]int{3, 2, 1})

				result, err := allocator.SelectDevices([]string{}, 6)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(6))

				bridgeAssignment := getBridgeAssignment(allocator, result)
				Expect(len(bridgeAssignment)).To(Equal(3), "Should use all 3 bridges")

				Expect(bridgeAssignment["numa0_bridgebridge0"]).To(Equal(3))
				Expect(bridgeAssignment["numa0_bridgebridge1"]).To(Equal(2))
				Expect(bridgeAssignment["numa0_bridgebridge2"]).To(Equal(1))
			})
		})
	})

	Describe("Smallest Bridge Selection for Partial Allocation", func() {
		Context("when same bridge count but different partial allocation choices", func() {
			It("should prefer smaller bridge for partial allocation within same bridge count", func() {
				// Bridge capacities: [3, 3, 2] - for allocation size 4
				// Both options use 2 bridges:
				// Option 1: bridge0(3) + bridge2(1/2) - 1 device remaining in bridge2 (smaller)
				// Option 2: bridge1(3) + bridge2(1/2) - 1 device remaining in bridge2 (same)
				// Since both have same pattern, algorithm should consistently pick one
				allocator := createTestTopologyWithBridges([]int{3, 3, 2})

				result, err := allocator.SelectDevices([]string{}, 4)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(4))

				bridgeAssignment := getBridgeAssignment(allocator, result)

				// Should use exactly 2 bridges (minimum possible for size 4)
				Expect(len(bridgeAssignment)).To(Equal(2), "Should use exactly 2 bridges")
			})

			It("should verify single bridge allocation when possible", func() {
				// Bridge capacities: [5, 3, 2] - for allocation size 4
				// Should prefer single bridge (bridge0) over multiple bridges
				allocator := createTestTopologyWithBridges([]int{5, 3, 2})

				result, err := allocator.SelectDevices([]string{}, 4)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(4))

				bridgeAssignment := getBridgeAssignment(allocator, result)

				// Should use only 1 bridge (bridge minimization priority)
				Expect(len(bridgeAssignment)).To(Equal(1), "Should use only 1 bridge due to bridge minimization")

				// Should use bridge0 with 4 devices allocated
				Expect(bridgeAssignment["numa0_bridgebridge0"]).To(Equal(4))
			})
		})
	})

	Describe("Complex Scenarios", func() {
		Context("with various bridge capacity distributions", func() {
			It("should handle uniform capacity bridges efficiently", func() {
				// Bridge capacities: [2, 2, 2, 2] - for allocation size 5
				// Should use 3 bridges: 2+2+1
				allocator := createTestTopologyWithBridges([]int{2, 2, 2, 2})

				result, err := allocator.SelectDevices([]string{}, 5)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(HaveLen(5))

				bridgeAssignment := getBridgeAssignment(allocator, result)
				Expect(len(bridgeAssignment)).To(Equal(3), "Should use exactly 3 bridges")
			})
		})
	})

	Describe("Error Scenarios", func() {
		Context("when insufficient devices are available", func() {
			It("should return error for allocation exceeding total capacity", func() {
				// Bridge capacities: [3, 2, 1] - total 6 devices, request 8
				allocator := createTestTopologyWithBridges([]int{3, 2, 1})

				result, err := allocator.SelectDevices([]string{}, 8)

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
				// NUMA 노드 레벨에서 먼저 실패하므로 해당 에러 메시지 확인
				Expect(err.Error()).To(ContainSubstring("no NUMA node with sufficient devices"))
			})
		})
	})
})
