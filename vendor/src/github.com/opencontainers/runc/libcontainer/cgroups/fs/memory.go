// +build linux

package fs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

type MemoryGroup struct {
}

func (s *MemoryGroup) Name() string {
	return "memory"
}

func (s *MemoryGroup) Apply(d *cgroupData) (err error) {
	fmt.Println("Enter MemoryGroup Apply")
	path, err := d.path("memory")
	if err != nil && !cgroups.IsNotFound(err) && !cgroups.IsV2Error(err) {
		fmt.Println("MemoryGroup Apply path path:", path, "err:", err)
		return err
	}
	fmt.Println("MemoryGroup After path path:", path)
	if memoryAssigned(d.config) {
		if path != "" {
			if subErr := os.MkdirAll(path, 0755); subErr != nil {
				fmt.Println("MemoryGroup Apply MkdirAll path:", path, "err:", subErr)
				return subErr
			}
			if cgroups.IsV2Error(err) {
				if subErr := d.addControllerForV2("memory", path); subErr != nil {
					return subErr
				}
			}
		}
		fmt.Println("MemoryGroup After MkdirAll path:", path)

		if err := s.Set(path, d.config); err != nil {
			fmt.Println("MemoryGroup Apply Set path:", path, "err:", err)
			return err
		}
		fmt.Println("MemoryGroup After Set path:", path)
	}

	defer func() {
		if err != nil {
			os.RemoveAll(path)
		}
	}()

	// We need to join memory cgroup after set memory limits, because
	// kmem.limit_in_bytes can only be set when the cgroup is empty.
	_, err = d.join("memory")
	if err != nil && !cgroups.IsNotFound(err) && !cgroups.IsV2Error(err) {
		fmt.Println("MemoryGroup Apply join path:", path, "err:", err)
		return err
	}

	return nil
}

func (s *MemoryGroup) Set(path string, cgroup *configs.Cgroup) error {
	if cgroup.Resources.Memory != 0 {
		if err := writeFile(path, "memory.max", strconv.FormatInt(cgroup.Resources.Memory, 10)); err != nil {
			return err
		}
	}
	if cgroup.Resources.MemoryReservation != 0 {
		if err := writeFile(path, "memory.low", strconv.FormatInt(cgroup.Resources.MemoryReservation, 10)); err != nil {
			return err
		}
	}
	if cgroup.Resources.MemorySwap > 0 {
		if err := writeFile(path, "memory.swap.max", strconv.FormatInt(cgroup.Resources.MemorySwap, 10)); err != nil {
			return err
		}
	}

	return nil
}

func (s *MemoryGroup) Remove(d *cgroupData) error {
	path, err := d.path("memory")
	if cgroups.IsV2Error(err) {
		err = nil
	}
	return removePath(path, err)
}

func (s *MemoryGroup) GetStats(path string, stats *cgroups.Stats) error {
	// Set stats from memory.stat.
	statsFile, err := os.Open(filepath.Join(path, "memory.stat"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer statsFile.Close()

	sc := bufio.NewScanner(statsFile)
	for sc.Scan() {
		t, v, err := getCgroupParamKeyValue(sc.Text())
		if err != nil {
			return fmt.Errorf("failed to parse memory.stat (%q) - %v", sc.Text(), err)
		}
		stats.MemoryStats.Stats[t] = v
	}
	stats.MemoryStats.Cache = stats.MemoryStats.Stats["file"]

	memoryUsage, err := getMemoryData(path, "")
	if err != nil {
		return err
	}
	stats.MemoryStats.Usage = memoryUsage
	swapUsage, err := getMemoryData(path, "swap")
	if err != nil {
		return err
	}
	stats.MemoryStats.SwapUsage = swapUsage

	return nil
}

func memoryAssigned(cgroup *configs.Cgroup) bool {
	return cgroup.Resources.Memory != 0 ||
		cgroup.Resources.MemoryReservation != 0 ||
		cgroup.Resources.MemorySwap > 0 ||
		cgroup.Resources.KernelMemory > 0 ||
		cgroup.Resources.OomKillDisable ||
		cgroup.Resources.MemorySwappiness != -1
}

func getMemoryData(path, name string) (cgroups.MemoryData, error) {
	memoryData := cgroups.MemoryData{}

	moduleName := "memory"
	if name != "" {
		moduleName = strings.Join([]string{"memory", name}, ".")
	}
	usage := strings.Join([]string{moduleName, "current"}, ".")

	value, err := getCgroupParamUint(path, usage)
	if err != nil {
		if moduleName != "memory" && os.IsNotExist(err) {
			return cgroups.MemoryData{}, nil
		}
		return cgroups.MemoryData{}, fmt.Errorf("failed to parse %s - %v", usage, err)
	}
	memoryData.Usage = value

	return memoryData, nil
}
