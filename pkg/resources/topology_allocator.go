package resources

import (
	"fmt"
	"sort"

	"github.com/golang/glog"
	"github.com/rebellions-sw/rebel-k8s-device-plugin/pkg/utils"
)

type NUMANode int

type BridgeID string

type Allocator interface {
	SelectDevices(mustIncludeDeviceIDs []string, allocationSize int) ([]string, error)
}

type TopologyAllocator struct {
	NodeTopology *NodeTopology
}

// PCIBridgeGroup represents devices connected to the same PCI bridge
type PCIBridgeGroup struct {
	DeviceIDs []string // Device IDs connected to this switch
}

// NUMATopology represents NUMA node topology information
type NUMATopology struct {
	PCIBridges map[BridgeID]*PCIBridgeGroup // PCI bridges in this NUMA node
}

// NodeTopology represents the complete topology information of a node
type NodeTopology struct {
	NUMANodes            map[NUMANode]*NUMATopology // NUMA nodes in this node
	PciBridgeDeviceCount int
	NumaNodeDeviceCount  int
}

type bridgeInfo struct {
	id      BridgeID
	free    int
	devices []string
}

type AllocationResult struct {
	selectedDevices        []string
	fullyFilledBridgeCount int
	deviceRemaining        int
}

type KubeletDelegatingAllocator struct {
	productID string
}

func (a *KubeletDelegatingAllocator) SelectDevices(mustIncludeDeviceIDs []string, allocationSize int) ([]string, error) {
	return nil, fmt.Errorf(
		"topology-aware allocation not supported for this product: %s, kubelet should handle device allocation",
		a.productID,
	)
}

func NewTopologyAllocator(
	availableDeviceIDs []string,
	bridgeIndex, pciBridgeDeviceCount, numaNodeDeviceCount int,
) (*TopologyAllocator, error) {
	topology := &NodeTopology{
		NUMANodes:            make(map[NUMANode]*NUMATopology),
		PciBridgeDeviceCount: pciBridgeDeviceCount,
		NumaNodeDeviceCount:  numaNodeDeviceCount,
	}

	for _, deviceID := range availableDeviceIDs {
		numaNode := utils.GetDevNode(deviceID)
		pciSwitch, err := utils.GetPCIBridge(deviceID, bridgeIndex)
		if err != nil {
			glog.Errorf("Failed to get PCI switch for device %s: %v", deviceID, err)
			return nil, err
		}

		if _, exists := topology.NUMANodes[NUMANode(numaNode)]; !exists {
			topology.NUMANodes[NUMANode(numaNode)] = &NUMATopology{
				PCIBridges: make(map[BridgeID]*PCIBridgeGroup),
			}
		}

		if _, exists := topology.NUMANodes[NUMANode(numaNode)].PCIBridges[BridgeID(pciSwitch)]; !exists {
			topology.NUMANodes[NUMANode(numaNode)].PCIBridges[BridgeID(pciSwitch)] = &PCIBridgeGroup{
				DeviceIDs: make([]string, 0),
			}
		}

		topology.NUMANodes[NUMANode(numaNode)].PCIBridges[BridgeID(pciSwitch)].DeviceIDs = append(
			topology.NUMANodes[NUMANode(numaNode)].PCIBridges[BridgeID(pciSwitch)].DeviceIDs,
			deviceID,
		)
	}

	return &TopologyAllocator{NodeTopology: topology}, nil
}

func selectOptimalNUMANode(ta *TopologyAllocator, allocationSize int) (*NUMATopology, error) {
	var selectedNUMA *NUMATopology
	minDeviceCountDiff := int(^uint(0) >> 1)

	for _, numaNode := range ta.NodeTopology.NUMANodes {
		numaDevices := ta.getAllDevicesFromNUMA(numaNode)
		availableDeviceCount := len(numaDevices)

		if availableDeviceCount >= allocationSize {
			deviceCountDiff := availableDeviceCount - allocationSize
			if deviceCountDiff < minDeviceCountDiff {
				minDeviceCountDiff = deviceCountDiff
				selectedNUMA = numaNode
			}
		}
	}

	if selectedNUMA == nil {
		return nil, fmt.Errorf("no NUMA node with sufficient devices (>= %d) found, topology-aware allocation failed", allocationSize)
	}
	return selectedNUMA, nil
}

func prepareBridgesSortedByCapacity(selectedNUMA *NUMATopology) []bridgeInfo {
	bridgeList := make([]bridgeInfo, 0)
	for bridgeID, bridgeGroup := range selectedNUMA.PCIBridges {
		bridgeList = append(bridgeList, bridgeInfo{
			id:      bridgeID,
			free:    len(bridgeGroup.DeviceIDs),
			devices: bridgeGroup.DeviceIDs,
		})
	}
	sort.Slice(bridgeList, func(i, j int) bool {
		return bridgeList[i].free > bridgeList[j].free
	})
	return bridgeList
}

func (ta *TopologyAllocator) SelectDevices(mustIncludeDeviceIDs []string, allocationSize int) ([]string, error) {
	if allocationSize > ta.NodeTopology.NumaNodeDeviceCount {
		return nil, fmt.Errorf(
			"allocation size %d exceeds NUMA node capacity %d, topology-aware allocation not beneficial, kubelet should handle allocation",
			allocationSize,
			ta.NodeTopology.NumaNodeDeviceCount,
		)
	}

	optimalNUMA, err := selectOptimalNUMANode(ta, allocationSize)
	if err != nil {
		return nil, err
	}

	glog.Infof("Selected NUMA node with %d free devices for allocation of %d", len(ta.getAllDevicesFromNUMA(optimalNUMA)), allocationSize)

	bridgeList := prepareBridgesSortedByCapacity(optimalNUMA)

	glog.Infof("Starting optimal allocation: need %d devices from %d bridges", allocationSize, len(bridgeList))

	allocationResult := findOptimalBridgeAllocation(bridgeList, allocationSize)
	if allocationResult == nil {
		return nil, fmt.Errorf("optimal allocation failed: no valid bridge combination found")
	}

	glog.Infof("Selecting Devices completed: selected devices: %v", allocationResult.selectedDevices)

	return allocationResult.selectedDevices, nil
}

func (ta *TopologyAllocator) getAllDevicesFromNUMA(numaNode *NUMATopology) []string {
	var allDeviceIDs []string
	for _, bridgeGroup := range numaNode.PCIBridges {
		allDeviceIDs = append(allDeviceIDs, bridgeGroup.DeviceIDs...)
	}
	return allDeviceIDs
}

func findOptimalBridgeAllocation(bridgeList []bridgeInfo, allocationSize int) *AllocationResult {
	for bridgeCount := 1; bridgeCount <= len(bridgeList); bridgeCount++ {
		glog.Infof("Trying allocation with %d bridges", bridgeCount)

		if allocationResult := findBestAllocationWithNBridges(bridgeList, allocationSize, bridgeCount); allocationResult != nil {
			return allocationResult
		}
	}
	return nil
}

func findBestAllocationWithNBridges(bridgeList []bridgeInfo, allocationSize, bridgeCount int) *AllocationResult {
	var bestAllocation *AllocationResult

	bridgeCombinations := generateBridgeCombinations(bridgeList, bridgeCount)

	for _, bridgeCombination := range bridgeCombinations {
		totalAvailableDevices := 0
		for _, bridge := range bridgeCombination {
			totalAvailableDevices += bridge.free
		}

		if totalAvailableDevices < allocationSize {
			continue
		}

		allocationResult := simulateDeviceAllocation(bridgeCombination, allocationSize)
		if allocationResult != nil {
			if isAllocationBetter(allocationResult, bestAllocation) {
				bestAllocation = allocationResult
			}
		}
	}

	return bestAllocation
}

func generateBridgeCombinations(bridgeList []bridgeInfo, targetBridgeCount int) [][]bridgeInfo {
	var combinationResults [][]bridgeInfo

	var backtrackCombinations func(startIndex int, currentCombination []bridgeInfo)
	backtrackCombinations = func(startIndex int, currentCombination []bridgeInfo) {
		if len(currentCombination) == targetBridgeCount {
			bridgeCombination := make([]bridgeInfo, len(currentCombination))
			copy(bridgeCombination, currentCombination)
			combinationResults = append(combinationResults, bridgeCombination)
			return
		}

		for bridgeIndex := startIndex; bridgeIndex < len(bridgeList); bridgeIndex++ {
			currentCombination = append(currentCombination, bridgeList[bridgeIndex])
			backtrackCombinations(bridgeIndex+1, currentCombination)
			currentCombination = currentCombination[:len(currentCombination)-1]
		}
	}

	backtrackCombinations(0, []bridgeInfo{})
	return combinationResults
}

func simulateDeviceAllocation(selectedBridges []bridgeInfo, allocationSize int) *AllocationResult {
	sort.Slice(selectedBridges, func(i, j int) bool {
		return selectedBridges[i].free > selectedBridges[j].free
	})

	if isTotalCapacityExactMatch(selectedBridges, allocationSize) {
		return handleExactCapacityMatch(selectedBridges)
	}

	return handlePartialBridgeAllocation(selectedBridges, allocationSize)
}

func isTotalCapacityExactMatch(selectedBridges []bridgeInfo, allocationSize int) bool {
	totalBridgeCapacity := 0
	for _, bridge := range selectedBridges {
		totalBridgeCapacity += bridge.free
	}
	return totalBridgeCapacity == allocationSize
}

func handleExactCapacityMatch(selectedBridges []bridgeInfo) *AllocationResult {
	var allocatedDevices []string

	for _, bridge := range selectedBridges {
		allocatedDevices = append(allocatedDevices, bridge.devices...)
	}

	return &AllocationResult{
		selectedDevices:        allocatedDevices,
		fullyFilledBridgeCount: len(selectedBridges),
		deviceRemaining:        0,
	}
}

func handlePartialBridgeAllocation(selectedBridges []bridgeInfo, allocationSize int) *AllocationResult {
	var allocatedDevices []string
	remainingToAllocate := allocationSize
	bridgesFullyFilled := 0
	devicesRemainingInPartialBridge := 0

	for bridgeIndex, bridge := range selectedBridges {
		if bridge.free <= remainingToAllocate {
			// Use entire bridge capacity
			allocatedDevices = append(allocatedDevices, bridge.devices...)
			remainingToAllocate -= bridge.free
			bridgesFullyFilled++
		} else {
			// Partial allocation needed - find smallest suitable bridge
			optimalPartialBridge := findSmallestSuitableBridge(selectedBridges, bridgeIndex, remainingToAllocate)
			allocatedDevices = append(allocatedDevices, optimalPartialBridge.devices[:remainingToAllocate]...)
			devicesRemainingInPartialBridge = optimalPartialBridge.free - remainingToAllocate
			break
		}
	}

	return &AllocationResult{
		selectedDevices:        allocatedDevices,
		fullyFilledBridgeCount: bridgesFullyFilled,
		deviceRemaining:        devicesRemainingInPartialBridge,
	}
}

func findSmallestSuitableBridge(bridgeCandidates []bridgeInfo, searchStartIndex, requiredCapacity int) bridgeInfo {
	optimalBridge := bridgeCandidates[searchStartIndex]

	for candidateIndex := searchStartIndex + 1; candidateIndex < len(bridgeCandidates); candidateIndex++ {
		candidate := bridgeCandidates[candidateIndex]
		if candidate.free < optimalBridge.free && candidate.free >= requiredCapacity {
			optimalBridge = candidate
		}
	}

	return optimalBridge
}

func isAllocationBetter(candidateAllocation, currentBestAllocation *AllocationResult) bool {
	if currentBestAllocation == nil {
		return true
	}

	if candidateAllocation.fullyFilledBridgeCount != currentBestAllocation.fullyFilledBridgeCount {
		return candidateAllocation.fullyFilledBridgeCount > currentBestAllocation.fullyFilledBridgeCount
	}

	return candidateAllocation.deviceRemaining > currentBestAllocation.deviceRemaining
}
