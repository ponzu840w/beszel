//go:build testing && freebsd

package zfs

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func fakeKstats(t *testing.T, pools []string, strs map[string]string, nums map[string]uint64) {
	oldString, oldUint64, oldNames := sysctlString, sysctlUint64, lastPoolNames.Load()
	t.Cleanup(func() {
		sysctlString, sysctlUint64 = oldString, oldUint64
		lastPoolNames.Store(oldNames)
	})
	sysctlString = func(name string) (string, error) {
		if v, ok := strs[name]; ok {
			return v, nil
		}
		return "", unix.ENOENT
	}
	sysctlUint64 = func(name string, _ ...int) (uint64, error) {
		if v, ok := nums[name]; ok {
			return v, nil
		}
		return 0, unix.ENOENT
	}
	stats := make([]PoolStat, len(pools))
	for i, name := range pools {
		stats[i].Name = name
	}
	rememberPoolNames(stats)
}

func iostats(pool string, arcRead, directRead, arcWrite, directWrite uint64) map[string]uint64 {
	prefix := "kstat.zfs." + pool + ".misc.iostats."
	return map[string]uint64{
		prefix + "arc_read_bytes": arcRead, prefix + "direct_read_bytes": directRead,
		prefix + "arc_write_bytes": arcWrite, prefix + "direct_write_bytes": directWrite,
	}
}

func TestPoolKernelStatsFreeBSD(t *testing.T) {
	fakeKstats(t, []string{"tank", "exported"},
		map[string]string{"kstat.zfs.tank.misc.state": "online\n"},
		iostats("tank", 100, 20, 300, 4))

	pools, err := PoolKernelStats()
	require.NoError(t, err)
	assert.Equal(t, []PoolKernelStat{{Name: "tank", Health: "ONLINE", NRead: 120, NWrite: 304}}, pools)
}

func TestPoolKernelStatsFreeBSDWithoutByteCounters(t *testing.T) {
	fakeKstats(t, []string{"tank"},
		map[string]string{"kstat.zfs.tank.misc.state": "ONLINE"},
		map[string]uint64{"kstat.zfs.tank.misc.iostats.trim_bytes_written": 0})

	_, err := PoolKernelStats()
	assert.ErrorIs(t, err, errors.ErrUnsupported)
}

func TestPoolKernelStatsFreeBSDWithoutPools(t *testing.T) {
	fakeKstats(t, []string{"exported"}, nil, nil)
	_, err := PoolKernelStats()
	assert.ErrorIs(t, err, ErrNoZfs)

	lastPoolNames.Store(nil)
	_, err = PoolKernelStats()
	assert.ErrorIs(t, err, ErrNoZfs)
}
