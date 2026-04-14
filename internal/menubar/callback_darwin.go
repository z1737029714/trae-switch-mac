//go:build darwin

package menubar

/*
#include <stdint.h>
*/
import "C"

//export TraeSwitchMenuBarHandleAction
func TraeSwitchMenuBarHandleAction(action C.int, providerIndex C.int) {
	dispatchAction(int(action), int(providerIndex))
}
