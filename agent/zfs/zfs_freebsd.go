//go:build freebsd

package zfs

import (
	"errors"
	"strings"

	"golang.org/x/sys/unix"
)

// Seams so tests can fake the kstat sysctl tree.
var (
	sysctlString = unix.Sysctl
	sysctlUint64 = unix.SysctlUint64
)

func ARCSize() (uint64, error) {
	return unix.SysctlUint64("kstat.zfs.misc.arcstats.size")
}

// PoolKernelStats reads the per-pool kstats that OpenZFS exports under
// kstat.zfs.<pool>.misc. They hold the same "state" and "iostats" counters as
// the Linux procfs files. Pools come from the last PoolStats call, which the
// storage pool backend always makes before sampling kernel stats.
func PoolKernelStats() ([]PoolKernelStat, error) {
	names := lastPoolNames.Load()
	if names == nil {
		return nil, ErrNoZfs
	}
	pools := make([]PoolKernelStat, 0, len(*names))
	for _, name := range *names {
		prefix := "kstat.zfs." + name + ".misc."
		state, err := sysctlString(prefix + "state")
		if errors.Is(err, unix.ENOENT) {
			continue // exported after the last PoolStats call
		}
		if err != nil {
			return nil, err
		}
		nread, nwrite, err := sysctlPoolIOStats(prefix + "iostats.")
		if err != nil {
			return nil, err
		}
		pools = append(pools, PoolKernelStat{
			Name: name, Health: strings.ToUpper(strings.TrimSpace(state)),
			NRead: nread, NWrite: nwrite,
		})
	}
	if len(pools) == 0 {
		return nil, ErrNoZfs
	}
	return pools, nil
}

// sysctlPoolIOStats sums the counters described at readPoolIOStats in
// zfs_linux.go. Before OpenZFS 2.3 the iostats kstat holds only TRIM
// statistics, and FreeBSD has no other pool-level byte counters.
func sysctlPoolIOStats(prefix string) (nread, nwrite uint64, err error) {
	var values [4]uint64
	for i, field := range [...]string{"arc_read_bytes", "direct_read_bytes", "arc_write_bytes", "direct_write_bytes"} {
		if values[i], err = sysctlUint64(prefix + field); err != nil {
			if errors.Is(err, unix.ENOENT) {
				err = errors.ErrUnsupported
			}
			return 0, 0, err
		}
	}
	return values[0] + values[1], values[2] + values[3], nil
}
