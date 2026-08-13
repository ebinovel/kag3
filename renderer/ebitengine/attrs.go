package ebitengine

import (
	"fmt"
	"strconv"
	"strings"
)

// getInt reads pm[key] as an int. ok reports whether the key was present at
// all; err is only ever non-nil when ok is true (an int-shaped attribute
// that failed to parse).
func getInt(pm map[string]string, key string) (n int, ok bool, err error) {
	v, ok := pm[key]
	if !ok {
		return 0, false, nil
	}
	n, err = strconv.Atoi(v)
	if err != nil {
		return 0, true, fmt.Errorf("[%s] の値 %q が int としてパースできません: %w", key, v, err)
	}
	return n, true, nil
}

// getBool reads pm[key] as a bool, same ok/err contract as getInt.
func getBool(pm map[string]string, key string) (b, ok bool, err error) {
	v, ok := pm[key]
	if !ok {
		return false, false, nil
	}
	b, err = strconv.ParseBool(v)
	if err != nil {
		return false, true, fmt.Errorf("[%s] の値 %q が bool としてパースできません: %w", key, v, err)
	}
	return b, true, nil
}

// getIntDefault reads pm[key] as an int, falling back to def when the key
// is absent.
func getIntDefault(pm map[string]string, key string, def int) (int, error) {
	n, ok, err := getInt(pm, key)
	if err != nil {
		return 0, err
	}
	if !ok {
		return def, nil
	}
	return n, nil
}

// getString reads pm[key] as-is; ok reports whether the key was present.
func getString(pm map[string]string, key string) (string, bool) {
	v, ok := pm[key]
	return v, ok
}

// parseColor accepts either a handful of named colors or a "0xRRGGBB" hex
// triplet (the two forms every bundled .ks color=/edge=/shadow=/
// border_color= attribute actually uses). The hex branch used to slice out
// only the first hex digit of each byte pair (e.g. "0x454D51" -> "4","4","5")
// and feed it to strconv.Atoi as a *decimal* number — "black" separately
// returned white (255,255,255) instead of black, and "white"/"pink" weren't
// recognized as named colors at all, falling into the same broken hex path
// and erroring (which panics the whole game, since any tag handler error
// propagates up through initScript's coroutine loop) or reading garbage
// runes past the string's end. Every one of these is actually used by the
// bundled example scripts (color="0x454D51"/"0xFAFAFA" for the custom
// message window, color="pink"/"white" elsewhere), so all were live bugs.
func parseColor(value string) (r, g, b int, err error) {
	switch value {
	case "black":
		return 0, 0, 0, nil
	case "white":
		return 255, 255, 255, nil
	case "red":
		return 255, 0, 0, nil
	case "blue":
		return 0, 0, 255, nil
	case "pink":
		return 255, 192, 203, nil
	}
	hex := strings.TrimPrefix(value, "0x")
	if len(hex) != 6 {
		return 0, 0, 0, fmt.Errorf("不正なカラーコードです: %s", value)
	}
	var v int64
	v, err = strconv.ParseInt(hex[0:2], 16, 32)
	if err != nil {
		return
	}
	r = int(v)
	v, err = strconv.ParseInt(hex[2:4], 16, 32)
	if err != nil {
		return
	}
	g = int(v)
	v, err = strconv.ParseInt(hex[4:6], 16, 32)
	if err != nil {
		return
	}
	b = int(v)
	return
}
