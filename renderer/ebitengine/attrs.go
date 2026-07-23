package ebitengine

import (
	"fmt"
	"strconv"
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
