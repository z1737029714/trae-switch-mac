package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"trae-switch/internal/cert"
	"trae-switch/internal/config"
	"trae-switch/internal/hosts"
	"trae-switch/internal/platform"
	"trae-switch/internal/portforward"
	"trae-switch/internal/privileged"
	"trae-switch/internal/proxy"
	"trae-switch/internal/truststore"
)

type App struct {
	ctx          context.Context
	certManager  *cert.CertificateManager
	hostsManager *hosts.HostsManager
	trustManager *truststore.TrustStoreManager
	proxyServer  *proxy.ProxyServer
	dataDir      string
	runtime      platform.Runtime
	runner       privileged.Runner
	access       *privileged.Access
	forwarder    *portforward.Manager
}

func NewApp() *App {
	return &App{
		runtime: platform.Current(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.runtime = platform.Current()

	dataDir, err := os.UserConfigDir()
	if err != nil {
		dataDir = os.Getenv("APPDATA")
	}
	if dataDir != "" {
		dataDir = filepath.Join(dataDir, "trae-switch")
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			log.Printf("Failed to create data directory: %v", err)
		}
	} else {
		dataDir = filepath.Join(os.TempDir(), "trae-switch")
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			log.Printf("Failed to create fallback data directory: %v", err)
		}
	}
	a.dataDir = dataDir

	a.certManager = cert.NewCertificateManager(dataDir)
	a.hostsManager = hosts.NewHostsManager()
	a.proxyServer = proxy.NewProxyServer("127.0.0.1", a.runtime.DefaultProxyPort())
	if a.runtime.GOOS() == "darwin" {
		a.runner = privileged.New()
		a.access = privileged.NewAccess(a.runner)
		a.forwarder = portforward.NewManager(dataDir, a.runtime.DefaultProxyPort(), a.runner, a.access)
	}

	if err := a.certManager.LoadOrGenerateCA(); err != nil {
		log.Printf("Failed to load/generate CA: %v", err)
	}

	a.trustManager = truststore.NewTrustStoreManager(a.certManager.GetCACertPath())

	if _, err := config.Load(); err != nil {
		log.Printf("Failed to load config: %v", err)
	}

	log.Println("Application started successfully")
}

func (a *App) GetStatus() map[string]interface{} {
	port := a.runtime.DefaultProxyPort()
	portAvailable, portProcess := proxy.CheckPortStatus(port)

	result := map[string]interface{}{
		"platform":             a.runtime.GOOS(),
		"requiresAdminRuntime": a.runtime.RequiresAdminRuntime(),
		"runningAsAdmin":       truststore.IsRunningAsAdmin() || !a.runtime.RequiresAdminRuntime(),
		"proxyRunning":         false,
		"proxyPort":            port,
		"hostsSet":             false,
		"certInstalled":        false,
		"portAvailable":        portAvailable,
		"portProcess":          portProcess,
		"portRedirectSet":      false,
		"macosTrusted":         false,
		"activeProvider":       nil,
		"activeTargetURL":      "",
		"providerReady":        false,
		"providerError":        "",
	}

	if a.hostsManager != nil {
		isSet, _ := a.hostsManager.IsSet()
		result["hostsSet"] = isSet
	}

	if a.trustManager != nil {
		isInstalled, _ := a.trustManager.IsInstalled()
		result["certInstalled"] = isInstalled
	}

	if a.proxyServer != nil {
		status := a.proxyServer.GetStatus()
		result["proxyRunning"] = status.Running
		result["activeTargetURL"] = status.TargetURL
		result["providerReady"] = status.ProviderReady
		result["providerError"] = status.ProviderError
	}

	if a.forwarder != nil {
		enabled, _ := a.forwarder.IsEnabled()
		result["portRedirectSet"] = enabled
	}

	if a.access != nil {
		result["macosTrusted"] = a.access.IsInstalled()
	}

	provider := config.GetActiveProvider()
	if provider != nil {
		result["activeProvider"] = map[string]interface{}{
			"name":   provider.Name,
			"models": provider.Models,
		}
	}

	return result
}

func (a *App) IsRunningAsAdmin() bool {
	return truststore.IsRunningAsAdmin() || !a.runtime.RequiresAdminRuntime()
}

func (a *App) SetHosts() error {
	if a.runtime.GOOS() == "darwin" {
		return a.updateHostsDarwin(true)
	}

	if a.runtime.RequiresAdminRuntime() && !truststore.IsRunningAsAdmin() {
		return fmt.Errorf("需要管理员权限")
	}
	return a.hostsManager.Set()
}

func (a *App) RestoreHosts() error {
	if a.runtime.GOOS() == "darwin" {
		return a.updateHostsDarwin(false)
	}

	if a.runtime.RequiresAdminRuntime() && !truststore.IsRunningAsAdmin() {
		return fmt.Errorf("需要管理员权限")
	}
	return a.hostsManager.Restore()
}

func (a *App) IsHostsSet() bool {
	isSet, _ := a.hostsManager.IsSet()
	return isSet
}

func (a *App) InstallCertificate() error {
	if a.runtime.RequiresAdminRuntime() && !truststore.IsRunningAsAdmin() {
		return fmt.Errorf("需要管理员权限")
	}

	if err := a.certManager.LoadOrGenerateCA(); err != nil {
		return fmt.Errorf("生成证书失败：%w", err)
	}

	return a.trustManager.Install()
}

func (a *App) InstallMacOSTrust() error {
	if a.runtime.GOOS() != "darwin" {
		return fmt.Errorf("仅支持 macOS")
	}
	if a.access == nil {
		return fmt.Errorf("macOS 一次性授权未初始化")
	}

	return a.access.Install()
}

func (a *App) UninstallCertificate() error {
	if a.runtime.RequiresAdminRuntime() && !truststore.IsRunningAsAdmin() {
		return fmt.Errorf("需要管理员权限")
	}
	return a.trustManager.Uninstall()
}

func (a *App) IsCertificateInstalled() bool {
	isInstalled, _ := a.trustManager.IsInstalled()
	return isInstalled
}

func (a *App) StartProxy() error {
	if a.runtime.RequiresAdminRuntime() && !truststore.IsRunningAsAdmin() {
		return fmt.Errorf("需要管理员权限监听 %d 端口", a.runtime.DefaultProxyPort())
	}

	if err := a.certManager.GenerateServerCert("api.openai.com"); err != nil {
		return fmt.Errorf("生成服务器证书失败：%w", err)
	}

	a.proxyServer.SetCertificate(
		a.certManager.GetServerCertPEM(),
		a.certManager.GetServerKeyPEM(),
	)

	if err := a.proxyServer.Start(a.ctx); err != nil {
		return err
	}

	if a.forwarder != nil {
		if err := a.forwarder.Enable(); err != nil {
			_ = a.proxyServer.Stop()
			return fmt.Errorf("启用 443 到 %d 的端口转发失败：%w", a.runtime.DefaultProxyPort(), err)
		}
	}

	return nil
}

func (a *App) StopProxy() error {
	var errs []string

	if err := a.proxyServer.Stop(); err != nil {
		errs = append(errs, err.Error())
	}

	if a.forwarder != nil {
		enabled, err := a.forwarder.IsEnabled()
		if err != nil {
			errs = append(errs, err.Error())
		} else if enabled {
			if err := a.forwarder.Disable(); err != nil {
				errs = append(errs, err.Error())
			}
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}

	return nil
}

func (a *App) IsProxyRunning() bool {
	return a.proxyServer != nil && a.proxyServer.IsRunning()
}

func (a *App) QuickStart() error {
	if a.runtime.RequiresAdminRuntime() && !truststore.IsRunningAsAdmin() {
		return fmt.Errorf("需要管理员权限")
	}

	if !a.IsHostsSet() {
		if err := a.SetHosts(); err != nil {
			return fmt.Errorf("设置 hosts 失败：%w", err)
		}
	}

	if !a.IsCertificateInstalled() {
		if err := a.InstallCertificate(); err != nil {
			return fmt.Errorf("安装证书失败：%w", err)
		}
	}

	if !a.IsProxyRunning() {
		if err := a.StartProxy(); err != nil {
			return fmt.Errorf("启动代理失败：%w", err)
		}
	}

	return nil
}

func (a *App) QuickStop() error {
	if a.IsProxyRunning() || a.forwarder != nil {
		if err := a.StopProxy(); err != nil {
			log.Printf("停止代理失败：%v", err)
		}
	}

	if a.IsHostsSet() {
		if err := a.RestoreHosts(); err != nil {
			log.Printf("恢复 hosts 失败：%v", err)
		}
	}

	return nil
}

func (a *App) GetProviders() []map[string]interface{} {
	providers := config.GetProviders()
	result := make([]map[string]interface{}, 0, len(providers))
	for i, p := range providers {
		result = append(result, map[string]interface{}{
			"index":       i,
			"name":        p.Name,
			"openai_base": p.OpenAIBase,
			"models":      p.Models,
		})
	}
	return result
}

func (a *App) GetActiveProviderIndex() int {
	return config.GetActiveProviderIndex()
}

func (a *App) SetActiveProvider(index int) error {
	return config.SetActiveProvider(index)
}

func (a *App) AddProvider(name, openaiBase string, models []string) error {
	provider := config.Provider{
		Name:       name,
		OpenAIBase: openaiBase,
		Models:     models,
	}
	return config.AddProvider(provider)
}

func (a *App) UpdateProvider(index int, name, openaiBase string, models []string) error {
	provider := config.Provider{
		Name:       name,
		OpenAIBase: openaiBase,
		Models:     models,
	}
	return config.UpdateProvider(index, provider)
}

func (a *App) DeleteProvider(index int) error {
	return config.DeleteProvider(index)
}

func (a *App) shutdown(ctx context.Context) {
	log.Println("Application shutting down...")
	if a.IsProxyRunning() || a.forwarder != nil {
		if err := a.StopProxy(); err != nil {
			log.Printf("Failed to stop proxy: %v", err)
		}
	}
	_ = ctx
}

func (a *App) updateHostsDarwin(set bool) error {
	if a.runner == nil {
		return fmt.Errorf("macOS 特权执行器未初始化")
	}

	current, err := a.hostsManager.ReadHosts()
	if err != nil {
		return err
	}

	var updated []byte
	if set {
		updated, err = a.hostsManager.BuildSetData(current)
	} else {
		updated, err = a.hostsManager.BuildRestoreData(current)
	}
	if err != nil {
		return err
	}

	stagingPath := ""
	if a.access != nil {
		stagingPath = a.access.HostsStagingPath()
	}
	if stagingPath == "" {
		stagingPath = filepath.Join(a.dataDir, "hosts.next")
	}
	if err := os.MkdirAll(filepath.Dir(stagingPath), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(stagingPath, updated, 0600); err != nil {
		return err
	}

	var (
		output []byte
		errRun error
	)
	if a.access != nil && a.access.IsInstalled() {
		output, errRun = privileged.RunWithoutPassword("/usr/bin/install", "-m", "0644", stagingPath, a.hostsManager.GetHostsPath())
	} else {
		output, errRun = a.runner.Run(fmt.Sprintf("install -m 0644 %q %q", stagingPath, a.hostsManager.GetHostsPath()))
	}
	if errRun != nil {
		return fmt.Errorf("更新 hosts 失败：%w: %s", errRun, strings.TrimSpace(string(output)))
	}

	return nil
}
