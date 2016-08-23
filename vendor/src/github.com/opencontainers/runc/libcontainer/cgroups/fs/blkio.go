// +build linux

package fs

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

type BlkioGroup struct {
}

func (s *BlkioGroup) Name() string {
	return "blkio"
}

func (s *BlkioGroup) Apply(d *cgroupData) error {
	fmt.Println("Enter BlkioGroup Apply")
	dir, err := d.join("blkio")
	if err != nil && !cgroups.IsNotFound(err) && !cgroups.IsV2Error(err) {
		fmt.Println("BlkioGroup Apply join dir:", dir, "err:", err)
		return err
	}

	if err := s.Set(dir, d.config); err != nil {
		fmt.Println("BlkioGroup Apply Set dir:", dir, "err:", err)
		return err
	}

	return nil
}

func (s *BlkioGroup) Set(path string, cgroup *configs.Cgroup) error {
	if cgroup.Resources.BlkioWeight != 0 {
		default_weight := "default " + strconv.FormatUint(uint64(cgroup.Resources.BlkioWeight), 10)
		if err := writeFile(path, "io.weight", default_weight); err != nil {
			return err
		}
	}

	for _, wd := range cgroup.Resources.BlkioWeightDevice {
		if err := writeFile(path, "io.weight", wd.WeightString()); err != nil {
			return err
		}
	}

	for _, td := range cgroup.Resources.BlkioThrottleReadBpsDevice {
		if err := writeFile(path, "io.max", "rbps "+td.String()); err != nil {
			return err
		}
	}

	for _, td := range cgroup.Resources.BlkioThrottleWriteBpsDevice {
		if err := writeFile(path, "io.max", "wbps "+td.String()); err != nil {
			return err
		}
	}

	for _, td := range cgroup.Resources.BlkioThrottleReadIOPSDevice {
		if err := writeFile(path, "io.max", "riops "+td.String()); err != nil {
			return err
		}
	}

	for _, td := range cgroup.Resources.BlkioThrottleWriteIOPSDevice {
		if err := writeFile(path, "io.max", "wiops "+td.String()); err != nil {
			return err
		}
	}

	return nil
}

func (s *BlkioGroup) Remove(d *cgroupData) error {
	path, err := d.path("blkio")
	if cgroups.IsV2Error(err) {
		err = nil
	}
	return removePath(path, err)
}

/*
examples:

    blkio.sectors
    8:0 6792

    blkio.io_service_bytes
    8:0 Read 1282048
    8:0 Write 2195456
    8:0 Sync 2195456
    8:0 Async 1282048
    8:0 Total 3477504
    Total 3477504

    blkio.io_serviced
    8:0 Read 124
    8:0 Write 104
    8:0 Sync 104
    8:0 Async 124
    8:0 Total 228
    Total 228

    blkio.io_queued
    8:0 Read 0
    8:0 Write 0
    8:0 Sync 0
    8:0 Async 0
    8:0 Total 0
    Total 0
*/

func splitBlkioStatLine(r rune) bool {
	return r == ' ' || r == ':'
}

func getBlkioStat(path string) ([]cgroups.BlkioStatEntry, error) {
	var blkioStats []cgroups.BlkioStatEntry
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return blkioStats, nil
		}
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		// format: dev type amount
		fields := strings.FieldsFunc(sc.Text(), splitBlkioStatLine)
		if len(fields) < 3 {
			if len(fields) == 2 && fields[0] == "Total" {
				// skip total line
				continue
			} else {
				return nil, fmt.Errorf("Invalid line found while parsing %s: %s", path, sc.Text())
			}
		}

		v, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return nil, err
		}
		major := v

		v, err = strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return nil, err
		}
		minor := v

		op := ""
		valueField := 2
		if len(fields) == 4 {
			op = fields[2]
			valueField = 3
		}
		v, err = strconv.ParseUint(fields[valueField], 10, 64)
		if err != nil {
			return nil, err
		}
		blkioStats = append(blkioStats, cgroups.BlkioStatEntry{Major: major, Minor: minor, Op: op, Value: v})
	}

	return blkioStats, nil
}

func (s *BlkioGroup) GetStats(path string, stats *cgroups.Stats) error {
	return getStats(path, stats) // Use generic stats as fallback
}

func getStats(path string, stats *cgroups.Stats) error {

	return nil
}
