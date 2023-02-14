package infoprovider

import (
	"fmt"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/k8snetworkplumbingwg/sriov-network-device-plugin/pkg/types"
)

type rebellionsInfoProvider struct {
	deviceID uint8
}

// NewRebellionsInfoProvider instantiate a generic DeviceInfoProvider
func NewRebellionsInfoProvider(pciBusID string) types.DeviceInfoProvider {
	// TODO(hjkim): get deviceID from pciBusID
	deviceID := uint8(0)
	return &rebellionsInfoProvider{
		deviceID: deviceID,
	}
}

func (rp *rebellionsInfoProvider) GetDeviceSpecs() []*pluginapi.DeviceSpec {
	devSpecs := make([]*pluginapi.DeviceSpec, 0)
	devicePath := fmt.Sprintf("/dev/rl%d", rp.deviceID)
	devSpecs = append(devSpecs, &pluginapi.DeviceSpec{
		HostPath:      devicePath,
		ContainerPath: devicePath,
		Permissions:   "rwm",
	})
	return devSpecs
}

func (rp *rebellionsInfoProvider) GetEnvVal() string {
	return ""
}

func (rp *rebellionsInfoProvider) GetMounts() []*pluginapi.Mount {
	mounts := make([]*pluginapi.Mount, 0)
	return mounts
}
