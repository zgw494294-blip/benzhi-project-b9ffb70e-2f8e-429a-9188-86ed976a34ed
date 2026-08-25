package main

import "testing"

func TestResolveAddress(t *testing.T) {
	got, err := resolveAddress("127.0.0.1:19137", true)
	if err != nil || got != "127.0.0.1:19137" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	for _, bad := range []string{"0.0.0.0:19137", "127.0.0.1:0", "localhost:19137", "127.0.0.1"} {
		if _, err = resolveAddress(bad, true); err == nil {
			t.Errorf("地址 %q 应被拒绝", bad)
		}
	}
}
