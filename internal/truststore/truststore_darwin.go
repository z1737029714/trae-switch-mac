//go:build darwin

package truststore

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

var (
	ErrNotAdmin = fmt.Errorf("administrator privileges required")
)

const certificateCommonName = "Trae Switch Root CA"

type TrustStoreManager struct {
	certPath string
}

func NewTrustStoreManager(certPath string) *TrustStoreManager {
	return &TrustStoreManager{
		certPath: certPath,
	}
}

func (tm *TrustStoreManager) IsInstalled() (bool, error) {
	_, err := runSecurity(tm.findCertificateArgs()...)
	if err != nil {
		return false, nil
	}

	return true, nil
}

func (tm *TrustStoreManager) Install() error {
	installed, err := tm.IsInstalled()
	if err != nil {
		return err
	}
	if installed {
		return nil
	}

	if _, err := runSecurity(tm.addTrustedCertArgs()...); err != nil {
		return fmt.Errorf("failed to install certificate: %w", err)
	}

	return nil
}

func (tm *TrustStoreManager) Uninstall() error {
	installed, err := tm.IsInstalled()
	if err != nil {
		return err
	}
	if !installed {
		return nil
	}

	if _, err := runSecurity(tm.removeTrustedCertArgs()...); err != nil {
		return fmt.Errorf("failed to uninstall certificate: %w", err)
	}

	return nil
}

func (tm *TrustStoreManager) addTrustedCertArgs() []string {
	return []string{
		"add-trusted-cert",
		"-r",
		"trustRoot",
		"-k",
		loginKeychainPath(),
		tm.certPath,
	}
}

func (tm *TrustStoreManager) findCertificateArgs() []string {
	return []string{
		"find-certificate",
		"-c",
		certificateCommonName,
		loginKeychainPath(),
	}
}

func (tm *TrustStoreManager) removeTrustedCertArgs() []string {
	return []string{
		"remove-trusted-cert",
		tm.certPath,
	}
}

func loginKeychainPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = os.Getenv("HOME")
	}

	return filepath.Join(home, "Library/Keychains/login.keychain-db")
}

func runSecurity(args ...string) ([]byte, error) {
	cmd := exec.Command("security", args...)
	return cmd.CombinedOutput()
}

func IsRunningAsAdmin() bool {
	return os.Geteuid() == 0
}
