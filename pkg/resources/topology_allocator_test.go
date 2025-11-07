package resources

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TopologyAllocator", func() {

	// Unified helper function to create test topology
	createTestTopology := func(numaConfigs map[int][]int, pciBridgeDeviceCount, numaNodeDeviceCount int) *TopologyAllocator {
		topology := &NodeTopology{
			NUMANodes:            make(map[NUMANode]*NUMATopology),
			PciBridgeDeviceCount: pciBridgeDeviceCount,
			NumaNodeDeviceCount:  numaNodeDeviceCount,
		}

		deviceCounter := 0
		for numaIdx, bridgeCapacities := range numaConfigs {
			numaTopology := &NUMATopology{
				PCIBridges: make(map[BridgeID]*PCIBridgeGroup),
			}

			for bridgeIdx, capacity := range bridgeCapacities {
				bridgeID := BridgeID(fmt.Sprintf("bridge%d", bridgeIdx))
				deviceIDs := make([]string, capacity)

				for i := 0; i < capacity; i++ {
					if len(numaConfigs) == 1 {
						// Single NUMA mode - use simple device ID
						deviceIDs[i] = fmt.Sprintf("device%d", deviceCounter)
					} else {
						// Multi-NUMA mode - include NUMA index in device ID
						deviceIDs[i] = fmt.Sprintf("numa%d_device%d", numaIdx, deviceCounter)
					}
					deviceCounter++
				}

				numaTopology.PCIBridges[bridgeID] = &PCIBridgeGroup{
					DeviceIDs: deviceIDs,
				}
			}

			topology.NUMANodes[NUMANode(numaIdx)] = numaTopology
		}

		return &TopologyAllocator{NodeTopology: topology}
	}

	// Convenience wrapper for single NUMA tests (backward compatibility)
	createTestTopologyWithBridges := func(bridgeCapacities []int) *TopologyAllocator {
		numaConfigs := map[int][]int{0: bridgeCapacities}
		return createTestTopology(numaConfigs, 10, 100)
	}

	// Convenience wrapper for multi-NUMA tests
	createTestTopologyWithMultipleNUMA := func(numaConfigs map[int][]int) *TopologyAllocator {
		return createTestTopology(numaConfigs, 10, 6) // NumaNodeDeviceCount를 6으로 변경
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

				result := allocator.SelectDevices([]string{}, 4)

				Expect(result).To(HaveLen(4))

				bridgeAssignment := getBridgeAssignment(allocator, result)
				Expect(len(bridgeAssignment)).To(Equal(1), "Should use only 1 bridge")

				// Should use bridge0 which has exactly 4 devices
				Expect(bridgeAssignment["numa0_bridgebridge0"]).To(Equal(4))
			})

			It("should prefer single bridge over multiple bridges for allocation size 3", func() {
				// Bridge capacities: [4, 3, 2, 1] - should select bridge1
				allocator := createTestTopologyWithBridges([]int{4, 3, 2, 1})

				result := allocator.SelectDevices([]string{}, 3)

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

				result := allocator.SelectDevices([]string{}, 6)

				Expect(result).To(HaveLen(6))

				bridgeAssignment := getBridgeAssignment(allocator, result)
				Expect(len(bridgeAssignment)).To(Equal(2), "Should use exactly 2 bridges")

				Expect(bridgeAssignment["numa0_bridgebridge0"]).To(Equal(4))
				Expect(bridgeAssignment["numa0_bridgebridge2"]).To(Equal(2))
			})

			It("should use optimal multiple bridge combination for allocation size 8", func() {
				// Bridge capacities: [4, 3, 2, 1] - should use 3 bridges for size 8
				allocator := createTestTopologyWithBridges([]int{4, 3, 2, 1})

				result := allocator.SelectDevices([]string{}, 8)

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

				result := allocator.SelectDevices([]string{}, 6)

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

				result := allocator.SelectDevices([]string{}, 4)

				Expect(result).To(HaveLen(4))

				bridgeAssignment := getBridgeAssignment(allocator, result)

				// Should use exactly 2 bridges (minimum possible for size 4)
				Expect(len(bridgeAssignment)).To(Equal(2), "Should use exactly 2 bridges")
			})

			It("should verify single bridge allocation when possible", func() {
				// Bridge capacities: [5, 3, 2] - for allocation size 4
				// Should prefer single bridge (bridge0) over multiple bridges
				allocator := createTestTopologyWithBridges([]int{5, 3, 2})

				result := allocator.SelectDevices([]string{}, 4)

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

				result := allocator.SelectDevices([]string{}, 5)

				Expect(result).To(HaveLen(5))

				bridgeAssignment := getBridgeAssignment(allocator, result)
				Expect(len(bridgeAssignment)).To(Equal(3), "Should use exactly 3 bridges")
			})
		})
	})

	Describe("Cross-NUMA Allocation", func() {
		// Helper function to count devices per NUMA node in result
		countDevicesPerNUMA := func(selectedDevices []string) map[int]int {
			numaCount := make(map[int]int)
			for _, deviceID := range selectedDevices {
				// Extract NUMA node index from device ID (numa0_device1 -> 0)
				var numaIdx int
				fmt.Sscanf(deviceID, "numa%d_device%d", &numaIdx, new(int))
				numaCount[numaIdx]++
			}
			return numaCount
		}

		Context("when allocation size exceeds single NUMA capacity", func() {
			It("should allocate across multiple NUMA nodes starting from smallest", func() {
				// NUMA 0: 4 devices, NUMA 1: 6 devices, NUMA 2: 8 devices
				// For allocation size 10 (> 8), should prioritize NUMA 0 (smallest) first
				numaConfigs := map[int][]int{
					0: {2, 2}, // NUMA 0: 4 devices total
					1: {3, 3}, // NUMA 1: 6 devices total
					2: {4, 4}, // NUMA 2: 8 devices total
				}
				allocator := createTestTopologyWithMultipleNUMA(numaConfigs)

				result := allocator.SelectDevices([]string{}, 10)

				Expect(result).To(HaveLen(10))

				numaCount := countDevicesPerNUMA(result)

				// Should use all 4 devices from NUMA 0 (smallest), then 6 from NUMA 1
				Expect(numaCount[0]).To(Equal(4), "Should use all devices from smallest NUMA 0 first")
				Expect(numaCount[1]).To(Equal(6), "Should use remaining devices from NUMA 1")
				Expect(numaCount[2]).To(Equal(0), "Should not use devices from largest NUMA 2")
			})

			It("should handle exact cross-NUMA allocation", func() {
				// NUMA 0: 3 devices, NUMA 1: 5 devices
				// For allocation size 8, should use all devices from both NUMA nodes
				numaConfigs := map[int][]int{
					0: {3}, // NUMA 0: 3 devices
					1: {5}, // NUMA 1: 5 devices
				}
				allocator := createTestTopologyWithMultipleNUMA(numaConfigs)

				result := allocator.SelectDevices([]string{}, 8)

				Expect(result).To(HaveLen(8))

				numaCount := countDevicesPerNUMA(result)

				Expect(numaCount[0]).To(Equal(3), "Should use all 3 devices from NUMA 0")
				Expect(numaCount[1]).To(Equal(5), "Should use all 5 devices from NUMA 1")
			})

			It("should handle partial allocation from multiple NUMA nodes", func() {
				// NUMA 0: 2 devices, NUMA 1: 4 devices, NUMA 2: 6 devices
				// For allocation size 9, should use NUMA 0 (2) + NUMA 1 (4) + partial NUMA 2 (3)
				numaConfigs := map[int][]int{
					0: {2}, // NUMA 0: 2 devices
					1: {4}, // NUMA 1: 4 devices
					2: {6}, // NUMA 2: 6 devices
				}
				allocator := createTestTopologyWithMultipleNUMA(numaConfigs)

				result := allocator.SelectDevices([]string{}, 9)

				Expect(result).To(HaveLen(9))

				numaCount := countDevicesPerNUMA(result)

				Expect(numaCount[0]).To(Equal(2), "Should use all devices from smallest NUMA 0 first")
				Expect(numaCount[1]).To(Equal(4), "Should use all devices from next smallest NUMA 1")
				Expect(numaCount[2]).To(Equal(3), "Should use remaining devices from largest NUMA 2")
			})

			It("should handle allocation with uneven NUMA distribution", func() {
				// NUMA 0: 1 device, NUMA 1: 3 devices, NUMA 2: 10 devices
				// For allocation size 12, should use 1 + 3 + 6 devices (limited by NumaNodeDeviceCount=6)
				numaConfigs := map[int][]int{
					0: {1},    // NUMA 0: 1 device
					1: {3},    // NUMA 1: 3 devices
					2: {5, 5}, // NUMA 2: 10 devices
				}
				allocator := createTestTopologyWithMultipleNUMA(numaConfigs)

				result := allocator.SelectDevices([]string{}, 11)

				Expect(result).To(HaveLen(11))

				numaCount := countDevicesPerNUMA(result)

				Expect(numaCount[0]).To(Equal(1), "Should use all devices from smallest NUMA 0 first")
				Expect(numaCount[1]).To(Equal(3), "Should use all devices from next smallest NUMA 1")
				Expect(numaCount[2]).To(Equal(7), "Should use remaining devices from largest NUMA 2")
			})

			It("should verify device selection order within cross-NUMA allocation", func() {
				// NUMA 0: 2 devices, NUMA 1: 5 devices
				// For allocation size 7 (> 6), should prefer NUMA 0 entirely + 5 from NUMA 1
				numaConfigs := map[int][]int{
					0: {2}, // NUMA 0: 2 devices
					1: {5}, // NUMA 1: 5 devices
				}
				allocator := createTestTopologyWithMultipleNUMA(numaConfigs)

				result := allocator.SelectDevices([]string{}, 7)

				Expect(result).To(HaveLen(7))

				numaCount := countDevicesPerNUMA(result)

				// Should exhaust smaller NUMA first
				Expect(numaCount[1]).To(Equal(5), "Should use all 5 devices from NUMA 1")
				Expect(numaCount[0]).To(Equal(2), "Should use all 2 devices from NUMA 0")
			})
		})

		Context("when single NUMA can satisfy allocation", func() {
			It("should not trigger cross-NUMA allocation", func() {
				// NUMA 0: 4 devices, NUMA 1: 6 devices
				// For allocation size 5 (< 8), should use single NUMA allocation
				numaConfigs := map[int][]int{
					0: {4}, // NUMA 0: 4 devices
					1: {6}, // NUMA 1: 6 devices
				}
				allocator := createTestTopologyWithMultipleNUMA(numaConfigs)

				result := allocator.SelectDevices([]string{}, 5)

				Expect(result).To(HaveLen(5))

				numaCount := countDevicesPerNUMA(result)

				// Should use only one NUMA node (the one with 6 devices can satisfy 5)
				usedNUMACount := 0
				for _, count := range numaCount {
					if count > 0 {
						usedNUMACount++
					}
				}
				Expect(usedNUMACount).To(Equal(1), "Should use only one NUMA node for allocation <= NumaNodeDeviceCount")
			})
		})
	})
})
