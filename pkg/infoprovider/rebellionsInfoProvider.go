package infoprovider

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/golang/glog"

	"github.com/rebellions-sw/rebel-k8s-device-plugin/pkg/types"
)

// constants used by rebellions info provider
const (
	RebellionsVendorID  = "1eff"
	SysfsDriverPools    = "/sys/bus/pci/drivers/rebellions/%s/pools"
	SysDeviceIommuGroup = "/sys/bus/pci/devices/%s/iommu_group"
	CharDeviceNode      = "/dev/%s"
	RsdNode             = "/dev/rsd0"
	VFIOPath            = "/dev/vfio/vfio"
)

type rebellionsInfoProvider struct {
	driver   string
	deviceID string
}

// NewRebellionsInfoProvider returns a new Rebellions Information Provider
func NewRebellionsInfoProvider(pciAddress, driver string) types.DeviceInfoProvider {
	if driver == "vfio-pci" {
		linkPath := fmt.Sprintf(SysDeviceIommuGroup, pciAddress)
		group, err := getIommuGroup(linkPath, pciAddress)
		if err != nil {
			glog.Errorf("NewRebellionsInfoProvider(): Failed to read %s: %s", linkPath, err.Error())
			return nil
		}
		deviceId := "vfio/" + group
		glog.Infof("NewRebellionsInfoProvider(): PCI Address: %s, Device ID: %s", pciAddress, deviceId)
		return &rebellionsInfoProvider{
			driver:   driver,
			deviceID: deviceId,
		}
	}

	poolsFilePath := fmt.Sprintf(SysfsDriverPools, pciAddress)
	poolsFile, err := os.ReadFile(poolsFilePath)
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

func _GetDevicePath(deviceID string) string {
	return fmt.Sprintf(CharDeviceNode, deviceID)
}

func (rp *rebellionsInfoProvider) GetName() string {
	return "rebellions"
}

func (rp *rebellionsInfoProvider) GetDeviceSpecs() []*pluginapi.DeviceSpec {
	devSpecs := make([]*pluginapi.DeviceSpec, 0)
	devicePath := _GetDevicePath(rp.deviceID)
	devSpecs = append(devSpecs, &pluginapi.DeviceSpec{
		HostPath:      devicePath,
		ContainerPath: devicePath,
		Permissions:   "rw",
	})
	if rp.driver != "vfio-pci" {
		devSpecs = append(devSpecs, &pluginapi.DeviceSpec{
			HostPath:      RsdNode,
			ContainerPath: RsdNode,
			Permissions:   "rw",
		})
	} else {
		devSpecs = append(devSpecs, &pluginapi.DeviceSpec{
			HostPath:      VFIOPath,
			ContainerPath: VFIOPath,
			Permissions:   "rw",
		})
	}

	return devSpecs
}

func (rp *rebellionsInfoProvider) GetEnvVal() types.AdditionalInfo {
	envs := make(map[string]string, 0)
	// @oceanjoon: this env is not actually used, but added to align with another info providers
	envs["mount"] = _GetDevicePath(rp.deviceID)
	return envs
}

func (rp *rebellionsInfoProvider) GetMounts() []*pluginapi.Mount {
	mounts := make([]*pluginapi.Mount, 0)
	if rp.driver != "vfio-pci" {
		mounts = append(mounts, &pluginapi.Mount{
			HostPath:      "/usr/local/bin/rbln-stat",
			ContainerPath: "/usr/bin/rbln-stat",
			ReadOnly:      true,
		})
	}
	return mounts
}

func getIommuGroup(linkPath, pciAddress string) (string, error) {
	// read the symlink to get the iommu group
	path, err := os.Readlink(linkPath)
	if err != nil {
		return "", fmt.Errorf("failed to read iommu_group for device %s: %v", pciAddress, err)
	}
	// get iommu group name from the path
	_, group := filepath.Split(path)
	return group, nil
}
