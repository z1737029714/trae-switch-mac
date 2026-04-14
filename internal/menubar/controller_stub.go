//go:build !darwin

package menubar

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

type noopController struct{}

func New(actions Actions) Controller {
	_ = actions
	return &noopController{}
}

func (n *noopController) Start()   {}
func (n *noopController) Refresh() {}
func (n *noopController) Close()   {}
