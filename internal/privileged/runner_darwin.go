//go:build darwin

package privileged

import (
	"fmt"
	"os/exec"
	"strings"
)

type MacRunner struct{}

func New() Runner {
	return MacRunner{}
}

func (MacRunner) Run(command string) ([]byte, error) {
	return exec.Command("osascript", "-e", appleScriptForCommand(command)).CombinedOutput()
}

func appleScriptForCommand(command string) string {
	return fmt.Sprintf(`do shell script "%s" with administrator privileges`, escapeAppleScript(command))
}

func escapeAppleScript(command string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return replacer.Replace(command)
}
