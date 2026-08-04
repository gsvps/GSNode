package cli

import "os"

func IsRunMode() bool {
	for _, arg := range os.Args[1:] {
		if arg == "-run" {
			return true
		}
	}
	return false
}

func IsJSONMode() bool {
	for _, arg := range os.Args[1:] {
		if arg == "-json" {
			return true
		}
	}
	return false
}
