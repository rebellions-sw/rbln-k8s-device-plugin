package resources

import (
	"fmt"

	"github.com/golang/glog"
	"github.com/rebellions-sw/rebel-k8s-device-plugin/pkg/utils"
)

type NUMANode int

type BridgeID string

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
	NUMANodes map[NUMANode]*NUMATopology // NUMA nodes in this node
}

func NewTopologyAllocator(availableDeviceIDs []string) (*TopologyAllocator, error) {
	topology := &NodeTopology{
		NUMANodes: make(map[NUMANode]*NUMATopology),
	}

	for _, deviceID := range availableDeviceIDs {
		numaNode := utils.GetDevNode(deviceID)
		pciSwitch, err := utils.GetPCIBridge(deviceID)
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

func (ta *TopologyAllocator) SelectDevices(mustIncludeDeviceIDs []string, allocationSize int) ([]string, error) {
	result := make([]string, 0, allocationSize)

	// Find allocationSize devices on the same PCI bridge
	for _, numaNode := range ta.NodeTopology.NUMANodes {
		for _, bridge := range numaNode.PCIBridges {
			if len(bridge.DeviceIDs) >= allocationSize {
				for i := 0; i < allocationSize; i++ {
					result = append(result, bridge.DeviceIDs[i])
				}
				glog.Infof("Selected %d devices from same PCI bridge", allocationSize)
				return result, nil
			}
		}
	}

	// Find allocationSize devices within the same NUMA node
	for _, numaNode := range ta.NodeTopology.NUMANodes {
		availableDevices := ta.getAllDevicesFromNUMA(numaNode)
		if len(availableDevices) >= allocationSize {
			for i := 0; i < allocationSize; i++ {
				result = append(result, availableDevices[i])
			}
			glog.Infof("Selected %d devices from same NUMA node", allocationSize)
			return result, nil
		}
	}

	// 3. Unable to find enough devices – delegate allocation decision to the kubelet
	return nil, fmt.Errorf("insufficient devices for topology-aware allocation: need %d, available topology cannot satisfy", allocationSize)
}

// getAllDevicesFromNUMA returns all devices from a NUMA node
func (ta *TopologyAllocator) getAllDevicesFromNUMA(numaNode *NUMATopology) []string {
	var devices []string
	for _, bridge := range numaNode.PCIBridges {
		devices = append(devices, bridge.DeviceIDs...)
	}
	return devices
}
