package portforward

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"trae-switch/internal/privileged"
)

const (
	anchorName     = "com.apple/trae-switch"
	anchorFilePath = "/etc/pf.anchors/trae-switch"
	tokenFileName  = "pf.token"
)

var tokenPattern = regexp.MustCompile(`Token\s*:\s*([0-9]+)`)

type Manager struct {
	dataDir        string
	proxyPort      int
	runner         privileged.Runner
	access         *privileged.Access
	anchorFilePath string
	tokenFilePath  string
	rulesFilePath  string
}

func NewManager(dataDir string, proxyPort int, runner privileged.Runner, access *privileged.Access) *Manager {
	rulesFilePath := filepath.Join(dataDir, "pf.rules")
	if access != nil {
		rulesFilePath = access.PFRulesStagingPath()
	}

	return &Manager{
		dataDir:        dataDir,
		proxyPort:      proxyPort,
		runner:         runner,
		access:         access,
		anchorFilePath: anchorFilePath,
		tokenFilePath:  filepath.Join(dataDir, tokenFileName),
		rulesFilePath:  rulesFilePath,
	}
}

func (m *Manager) IsEnabled() (bool, error) {
	data, err := os.ReadFile(m.anchorFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return string(data) == m.renderRules(), nil
}

func (m *Manager) renderRules() string {
	return fmt.Sprintf("rdr pass on lo0 inet proto tcp from any to 127.0.0.1 port 443 -> 127.0.0.1 port %d\n", m.proxyPort)
}

func (m *Manager) Enable() error {
	if m.runner == nil {
		return fmt.Errorf("privileged runner is not configured")
	}

	if err := os.MkdirAll(filepath.Dir(m.rulesFilePath), 0700); err != nil {
		return err
	}

	if err := os.WriteFile(m.rulesFilePath, []byte(m.renderRules()), 0600); err != nil {
		return err
	}

	output, err := m.enablePrivilegedCommands()
	if err != nil {
		return fmt.Errorf("failed to enable port redirect: %w: %s", err, strings.TrimSpace(string(output)))
	}

	token := parseEnableToken(string(output))
	if token != "" {
		if err := os.WriteFile(m.tokenFilePath, []byte(token), 0600); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) Disable() error {
	if m.runner == nil {
		return fmt.Errorf("privileged runner is not configured")
	}

	token, _ := os.ReadFile(m.tokenFilePath)

	output, err := m.disablePrivilegedCommands(strings.TrimSpace(string(token)))
	if err != nil {
		return fmt.Errorf("failed to disable port redirect: %w: %s", err, strings.TrimSpace(string(output)))
	}

	if err := os.Remove(m.tokenFilePath); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (m *Manager) enablePrivilegedCommands() ([]byte, error) {
	if m.access != nil && m.access.IsInstalled() {
		if output, err := privileged.RunWithoutPassword("/usr/bin/install", "-m", "0644", m.rulesFilePath, m.anchorFilePath); err != nil {
			return output, err
		}
		if output, err := privileged.RunWithoutPassword("/sbin/pfctl", "-a", anchorName, "-f", m.anchorFilePath); err != nil {
			return output, err
		}

		return privileged.RunWithoutPassword("/sbin/pfctl", "-E")
	}

	return m.runner.Run(m.buildEnableCommand(m.rulesFilePath))
}

func (m *Manager) disablePrivilegedCommands(token string) ([]byte, error) {
	if m.access != nil && m.access.IsInstalled() {
		if output, err := privileged.RunWithoutPassword("/sbin/pfctl", "-a", anchorName, "-F", "all"); err != nil {
			return output, err
		}
		if output, err := privileged.RunWithoutPassword("/bin/rm", "-f", m.anchorFilePath); err != nil {
			return output, err
		}
		if token != "" {
			return privileged.RunWithoutPassword("/sbin/pfctl", "-X", token)
		}

		return []byte{}, nil
	}

	return m.runner.Run(m.buildDisableCommand(token))
}

func (m *Manager) buildEnableCommand(tempRulesPath string) string {
	return fmt.Sprintf(
		`mkdir -p %q && install -m 0644 %q %q && /sbin/pfctl -a %q -f %q && /sbin/pfctl -E`,
		filepath.Dir(m.anchorFilePath),
		tempRulesPath,
		m.anchorFilePath,
		anchorName,
		m.anchorFilePath,
	)
}

func (m *Manager) buildDisableCommand(token string) string {
	command := fmt.Sprintf(`/sbin/pfctl -a %q -F all || true; rm -f %q`, anchorName, m.anchorFilePath)
	if token != "" {
		command += fmt.Sprintf(`; /sbin/pfctl -X %q || true`, token)
	}

	return command
}

func parseEnableToken(output string) string {
	matches := tokenPattern.FindStringSubmatch(output)
	if len(matches) != 2 {
		return ""
	}

	return strings.TrimSpace(matches[1])
}
