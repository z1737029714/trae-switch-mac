//go:build darwin

package menubar

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>

void TraeSwitchMenuBarEnsure(void);
void TraeSwitchMenuBarRefresh(const char *stateJSON);
void TraeSwitchMenuBarShowError(const char *title, const char *message);
void TraeSwitchMenuBarClose(void);
*/
import "C"

import (
	"encoding/json"
	"log"
	"sync"
	"unsafe"
)

const (
	actionToggleProxy = 1
	actionShowWindow  = 2
	actionQuit        = 3
	actionSwitch      = 4
)

type Provider struct {
	Index int    `json:"index"`
	Name  string `json:"name"`
}

type State struct {
	ProxyRunning        bool       `json:"proxyRunning"`
	ActiveProviderIndex int        `json:"activeProviderIndex"`
	ActiveProviderName  string     `json:"activeProviderName"`
	Providers           []Provider `json:"providers"`
}

type Actions interface {
	GetMenuBarState() State
	StartProxy() error
	StopProxy() error
	SetActiveProvider(index int) error
	ShowMainWindow()
	QuitApp() error
}

type Controller interface {
	Start()
	Refresh()
	Close()
}

type controller struct {
	actions Actions
}

var (
	activeControllerMu sync.RWMutex
	activeController   *controller
)

func New(actions Actions) Controller {
	return &controller{actions: actions}
}

func (c *controller) Start() {
	if c == nil {
		return
	}

	activeControllerMu.Lock()
	activeController = c
	activeControllerMu.Unlock()

	C.TraeSwitchMenuBarEnsure()
	c.Refresh()
}

func (c *controller) Refresh() {
	if c == nil || c.actions == nil {
		return
	}

	stateJSON, err := json.Marshal(c.actions.GetMenuBarState())
	if err != nil {
		log.Printf("Failed to marshal menu bar state: %v", err)
		return
	}

	cStateJSON := C.CString(string(stateJSON))
	defer C.free(unsafe.Pointer(cStateJSON))

	C.TraeSwitchMenuBarRefresh(cStateJSON)
}

func (c *controller) Close() {
	activeControllerMu.Lock()
	if activeController == c {
		activeController = nil
	}
	activeControllerMu.Unlock()
	C.TraeSwitchMenuBarClose()
}

func dispatchAction(action int, providerIndex int) {
	activeControllerMu.RLock()
	controller := activeController
	activeControllerMu.RUnlock()

	if controller == nil || controller.actions == nil {
		return
	}

	go controller.handleAction(action, providerIndex)
}

func (c *controller) handleAction(action int, providerIndex int) {
	var err error

	switch action {
	case actionToggleProxy:
		state := c.actions.GetMenuBarState()
		if state.ProxyRunning {
			err = c.actions.StopProxy()
		} else {
			err = c.actions.StartProxy()
		}
	case actionShowWindow:
		c.actions.ShowMainWindow()
	case actionQuit:
		err = c.actions.QuitApp()
	case actionSwitch:
		err = c.actions.SetActiveProvider(providerIndex)
	}

	if err != nil {
		log.Printf("Menu bar action failed: %v", err)
		showError(err.Error())
	}

	c.Refresh()
}

func showError(message string) {
	cTitle := C.CString("Trae Switch")
	cMessage := C.CString(message)
	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cMessage))

	C.TraeSwitchMenuBarShowError(cTitle, cMessage)
}
