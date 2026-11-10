package app

import (
	"reflect"
	"unsafe"

	viva "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// effectInvalidator is the unexported invalidate field of ui.EffectState.
// Page.Config can replace the invalidator after the driver binds the one Run
// passed in. The effect copies the page's pointer when it is declared, and
// that pointer is the one the driver bound.
func effectInvalidator(state *ui.EffectState[loadResult, string]) *viva.Invalidator {
	if state == nil {
		return nil
	}
	field, ok := reflect.TypeOf(state).Elem().FieldByName("invalidate")
	if !ok || field.Type != reflect.TypeOf((*viva.Invalidator)(nil)) {
		return nil
	}
	return *(**viva.Invalidator)(unsafe.Add(unsafe.Pointer(state), field.Offset))
}
