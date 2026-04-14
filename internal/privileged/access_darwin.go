//go:build darwin

package privileged

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

const (
	sudoersPath        = "/etc/sudoers.d/trae-switch"
	sudoersTempPath    = "/etc/sudoers.d/trae-switch.tmp"
	trustedDirName     = ".trae-switch"
	hostsStagingName   = "hosts.next"
	pfRulesStagingName = "pf.rules"
)

type Access struct {
	username string
	workDir  string
	runner   Runner
}

func NewAccess(runner Runner) *Access {
	return &Access{
		username: currentUsername(),
		workDir:  defaultTrustedWorkDir(),
		runner:   runner,
	}
}

func (a *Access) WorkDir() string {
	return a.workDir
}

func (a *Access) HostsStagingPath() string {
	return filepath.Join(a.workDir, hostsStagingName)
}

func (a *Access) PFRulesStagingPath() string {
	return filepath.Join(a.workDir, pfRulesStagingName)
}

func (a *Access) IsInstalled() bool {
	if a == nil {
		return false
	}

	return commandAllowedWithoutPassword(a.passwordlessCheckArgs()...)
}

func (a *Access) Install() error {
	if a == nil || a.runner == nil {
		return fmt.Errorf("macOS trust installer is not configured")
	}

	if err := os.MkdirAll(a.workDir, 0700); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(a.workDir, "sudoers-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(renderSudoers(a.username, a.workDir)); err != nil {
		tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	output, err := a.runner.Run(buildInstallSudoersCommand(tmpFile.Name()))
	if err != nil {
		return fmt.Errorf("安装 macOS 一次性授权失败：%w: %s", err, strings.TrimSpace(string(output)))
	}

	return nil
}

func RunWithoutPassword(name string, args ...string) ([]byte, error) {
	commandArgs := append([]string{"-n", name}, args...)
	return exec.Command("sudo", commandArgs...).CombinedOutput()
}

func buildInstallSudoersCommand(tempPath string) string {
	return fmt.Sprintf(
		`mkdir -p %q && install -o root -g wheel -m 0440 %q %q && /usr/sbin/visudo -cf %q && mv %q %q`,
		filepath.Dir(sudoersPath),
		tempPath,
		sudoersTempPath,
		sudoersTempPath,
		sudoersTempPath,
		sudoersPath,
	)
}

func renderSudoers(username, workDir string) string {
	lines := []string{
		"# Trae Switch macOS one-time trust",
		fmt.Sprintf("%s ALL=(root) NOPASSWD: /usr/bin/install -m 0644 %s /etc/hosts", username, filepath.Join(workDir, hostsStagingName)),
		fmt.Sprintf("%s ALL=(root) NOPASSWD: /usr/bin/install -m 0644 %s /etc/pf.anchors/trae-switch", username, filepath.Join(workDir, pfRulesStagingName)),
		fmt.Sprintf("%s ALL=(root) NOPASSWD: /sbin/pfctl -a com.apple/trae-switch -f /etc/pf.anchors/trae-switch", username),
		fmt.Sprintf("%s ALL=(root) NOPASSWD: /sbin/pfctl -E", username),
		fmt.Sprintf("%s ALL=(root) NOPASSWD: /sbin/pfctl -a com.apple/trae-switch -F all", username),
		fmt.Sprintf("%s ALL=(root) NOPASSWD: /bin/rm -f /etc/pf.anchors/trae-switch", username),
		fmt.Sprintf("%s ALL=(root) NOPASSWD: /sbin/pfctl -X *", username),
	}

	return strings.Join(lines, "\n") + "\n"
}

func (a *Access) passwordlessCheckArgs() []string {
	return []string{
		"-n",
		"-l",
		"/usr/bin/install",
		"-m",
		"0644",
		a.HostsStagingPath(),
		"/etc/hosts",
	}
}

func commandAllowedWithoutPassword(args ...string) bool {
	cmd := exec.Command("sudo", args...)
	return cmd.Run() == nil
}

func defaultTrustedWorkDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = os.Getenv("HOME")
	}
	if home == "" {
		return filepath.Join(os.TempDir(), "trae-switch")
	}

	return filepath.Join(home, trustedDirName)
}

func currentUsername() string {
	currentUser, err := user.Current()
	if err == nil && currentUser.Username != "" {
		return currentUser.Username
	}

	if username := os.Getenv("USER"); username != "" {
		return username
	}

	return "unknown"
}

func containsLine(haystack, needle string) bool {
	for _, line := range strings.Split(haystack, "\n") {
		if line == needle {
			return true
		}
	}

	return false
}
