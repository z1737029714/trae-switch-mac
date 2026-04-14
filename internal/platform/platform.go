package platform

import "runtime"

type Runtime struct {
	goos string
}

func Current() Runtime {
	return NewRuntime(runtime.GOOS)
}

func NewRuntime(goos string) Runtime {
	return Runtime{goos: goos}
}

func (rt Runtime) GOOS() string {
	return rt.goos
}

func (rt Runtime) DefaultProxyPort() int {
	if rt.goos == "darwin" {
		return 8443
	}

	return 443
}

func (rt Runtime) RequiresAdminRuntime() bool {
	return rt.goos == "windows"
}
