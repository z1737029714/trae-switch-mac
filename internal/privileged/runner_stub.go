//go:build !darwin

package privileged

import "fmt"

type unsupportedRunner struct{}

func New() Runner {
	return unsupportedRunner{}
}

func (unsupportedRunner) Run(command string) ([]byte, error) {
	return nil, fmt.Errorf("privileged runner is not implemented on this platform: %s", command)
}
