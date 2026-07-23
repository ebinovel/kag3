package ebitengine

import "testing"

func TestGetIntPresent(t *testing.T) {
	n, ok, err := getInt(map[string]string{"x": "42"}, "x")
	if err != nil || !ok || n != 42 {
		t.Fatalf("getInt = %d, %v, %v; want 42, true, nil", n, ok, err)
	}
}

func TestGetIntAbsent(t *testing.T) {
	n, ok, err := getInt(map[string]string{}, "x")
	if err != nil || ok || n != 0 {
		t.Fatalf("getInt = %d, %v, %v; want 0, false, nil", n, ok, err)
	}
}

func TestGetIntInvalid(t *testing.T) {
	_, ok, err := getInt(map[string]string{"x": "abc"}, "x")
	if err == nil || !ok {
		t.Fatalf("getInt(%q) = ok=%v err=%v; want ok=true, err!=nil", "abc", ok, err)
	}
}

func TestGetBoolPresent(t *testing.T) {
	b, ok, err := getBool(map[string]string{"x": "true"}, "x")
	if err != nil || !ok || !b {
		t.Fatalf("getBool = %v, %v, %v; want true, true, nil", b, ok, err)
	}
}

func TestGetBoolAbsent(t *testing.T) {
	b, ok, err := getBool(map[string]string{}, "x")
	if err != nil || ok || b {
		t.Fatalf("getBool = %v, %v, %v; want false, false, nil", b, ok, err)
	}
}

func TestGetBoolInvalid(t *testing.T) {
	_, ok, err := getBool(map[string]string{"x": "notabool"}, "x")
	if err == nil || !ok {
		t.Fatalf("getBool(%q) = ok=%v err=%v; want ok=true, err!=nil", "notabool", ok, err)
	}
}

func TestGetIntDefaultPresent(t *testing.T) {
	n, err := getIntDefault(map[string]string{"x": "7"}, "x", 99)
	if err != nil || n != 7 {
		t.Fatalf("getIntDefault = %d, %v; want 7, nil", n, err)
	}
}

func TestGetIntDefaultAbsent(t *testing.T) {
	n, err := getIntDefault(map[string]string{}, "x", 99)
	if err != nil || n != 99 {
		t.Fatalf("getIntDefault = %d, %v; want 99, nil", n, err)
	}
}

func TestGetIntDefaultInvalid(t *testing.T) {
	_, err := getIntDefault(map[string]string{"x": "abc"}, "x", 99)
	if err == nil {
		t.Fatal("getIntDefault with invalid value: want error, got nil")
	}
}

func TestGetString(t *testing.T) {
	v, ok := getString(map[string]string{"x": "hello"}, "x")
	if !ok || v != "hello" {
		t.Fatalf("getString = %q, %v; want hello, true", v, ok)
	}
	v, ok = getString(map[string]string{}, "x")
	if ok || v != "" {
		t.Fatalf("getString = %q, %v; want \"\", false", v, ok)
	}
}
