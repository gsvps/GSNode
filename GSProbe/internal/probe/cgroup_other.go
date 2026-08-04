//go:build !linux

package probe

func collectCgroupLimitsLinux(base cgroupLimits) cgroupLimits {
	return base
}

func readProcFile(string) ([]byte, error) {
	return nil, nil
}
