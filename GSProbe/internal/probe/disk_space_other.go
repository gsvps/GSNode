//go:build !linux

package probe

type diskVolume struct {
	Total, Free uint64
	InodesTotal uint64
	InodesFree  uint64
	FSType      string
	BlockDevice string
	IOScheduler string
}

func diskSpace(string) (total, free uint64) { return 0, 0 }

func diskVolumeInfo(path string) diskVolume {
	return diskVolume{BlockDevice: diskDevice(path)}
}
