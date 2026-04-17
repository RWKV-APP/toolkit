package llm_toolkit

import (
	"os"
	"testing"
)

func TestLLMToolkit_GetHardwareInfo(t *testing.T) {
	info, err := GetHardwareInfo()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("hardware info: %s", info.JSONString())
}

func TestLLMToolkit_GetHardwareUsageInfo(t *testing.T) {
	info, err := GetHardwareUsageInfoForPID(int32(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	if info.Process == nil {
		t.Fatal("expected process info")
	}
	t.Logf("hardware usage info: %s", info.JSONString())
}
