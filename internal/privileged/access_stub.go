//go:build !darwin

package privileged

import "fmt"

type Access struct{}

func NewAccess(runner Runner) *Access {
	return nil
}

func (a *Access) WorkDir() string {
	return ""
}

func (a *Access) HostsStagingPath() string {
	return ""
}

func (a *Access) PFRulesStagingPath() string {
	return ""
}

func (a *Access) IsInstalled() bool {
	return false
}

func (a *Access) Install() error {
	return fmt.Errorf("macOS one-time trust is not supported on this platform")
}

func RunWithoutPassword(name string, args ...string) ([]byte, error) {
	return nil, fmt.Errorf("passwordless privileged execution is not supported on this platform: %s", name)
}
