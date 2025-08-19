package resources

import "github.com/rebellions-sw/rbln-k8s-device-plugin/pkg/types"

const (
	AtomCA25BridgeIndex          = 2
	AtomCA25PciBridgeDeviceCount = 4
	AtomCA25NumaNodeDeviceCount  = 16

	AtomCA22BridgeIndex          = 1
	AtomCA22PciBridgeDeviceCount = 4
	AtomCA22NumaNodeDeviceCount  = 8
)

func CreateAllocator(availableDeviceIDs []string, productID string, resourceConfig *types.ResourceConfig) (Allocator, error) {
	var allocator Allocator
	var err error

	switch productID {
	case "1251", "1250":
		allocator, err = NewTopologyAllocator(
			availableDeviceIDs,
			AtomCA25BridgeIndex,
			AtomCA25PciBridgeDeviceCount,
			AtomCA25NumaNodeDeviceCount,
			resourceConfig.AllowCrossNuma,
		)
		if err != nil {
			return nil, err
		}
		return allocator, nil
	case "1221", "1220":
		allocator, err = NewTopologyAllocator(
			availableDeviceIDs,
			AtomCA22BridgeIndex,
			AtomCA22PciBridgeDeviceCount,
			AtomCA22NumaNodeDeviceCount,
			resourceConfig.AllowCrossNuma,
		)
		if err != nil {
			return nil, err
		}
		return allocator, nil
	default:
		return &KubeletDelegatingAllocator{productID: productID}, nil
	}
}
