package infoprovider

import (
	"fmt"
	"io/ioutil"
	"strings"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/golang/glog"
	"github.com/k8snetworkplumbingwg/sriov-network-device-plugin/pkg/types"
)

const (
	SysfsDriverPools = "/sys/bus/pci/drivers/rebellions/%s/pools"
	CharDeviceNode   = "/dev/%s"
)

type rebellionsInfoProvider struct {
	deviceID string
}

func NewRebellionsInfoProvider(pciAddress string) types.DeviceInfoProvider {
	poolsFilePath := fmt.Sprintf(SysfsDriverPools, pciAddress)
	poolsFile, err := ioutil.ReadFile(poolsFilePath)
	if err != nil {
		glog.Errorf("NewRebellionsInfoProvider(): Failed to read %s: %s", poolsFilePath, err.Error())
		return nil
	}

	// `pools` file has contents as below
	// =====================================
	// poolinfo - 0.1
	// rbln0               0    4 1024  1
	// =====================================
	// so we take a second line and take the first one among words split by spaces
	deviceID := strings.Split(strings.Split(string(poolsFile), "\n")[1], " ")[0]
	glog.Infof("NewRebellionsInfoProvider(): PCI Address: %s, Device ID: %s", pciAddress, deviceID)

	return &rebellionsInfoProvider{
		deviceID: deviceID,
	}
}

func GetDevicePath(deviceID string) string {
	return fmt.Sprintf(CharDeviceNode, deviceID)
}

func (rp *rebellionsInfoProvider) GetName() string {
	return "rebellions"
}

func (rp *rebellionsInfoProvider) GetDeviceSpecs() []*pluginapi.DeviceSpec {
	devSpecs := make([]*pluginapi.DeviceSpec, 0)
	devicePath := GetDevicePath(rp.deviceID)
	devSpecs = append(devSpecs, &pluginapi.DeviceSpec{
		HostPath:      devicePath,
		ContainerPath: devicePath,
		Permissions:   "rwm",
	})
	return devSpecs
}

func (rp *rebellionsInfoProvider) GetEnvVal() types.AdditionalInfo {
	envs := make(map[string]string, 0)
	// @oceanjoon: this env is not actually used, but added to align with another info providers
	envs["mount"] = GetDevicePath(rp.deviceID)
	return envs
}

func (rp *rebellionsInfoProvider) GetMounts() []*pluginapi.Mount {
	mounts := make([]*pluginapi.Mount, 0)
	mounts = append(mounts, &pluginapi.Mount{
		HostPath:	"/usr/local/bin/rbln-stat",
		ContainerPath:  "/usr/bin/rbln-stat",
		ReadOnly:	true,
	})
	return mounts
}
