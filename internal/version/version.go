package version

import (
	"fmt"
	"runtime/debug"
)

var (
	defaultVersion = "UNSTABLE"
	version        = ""
)

func Version() string {
	if version != "" && version != defaultVersion {
		return version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Println(ok)
		return defaultVersion
	}

	if info.Main.Version == "(devel)" {
		fmt.Println(info.Main.Version)
		return defaultVersion
	}

	return info.Main.Version
}
