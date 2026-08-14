//go:build js && wasm

package ebitengine

import "syscall/js"

// openURLPlatform opens rawURL the way a browser tab actually can: calling
// the real window.open through syscall/js, same as storage_js.go's
// localStorage bridge — syscall/js only, no external deps.
func openURLPlatform(rawURL string) error {
	js.Global().Call("open", rawURL, "_blank")
	return nil
}
