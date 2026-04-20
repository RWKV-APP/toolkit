package llm_toolkit

import (
	"context"
	"os"
	"testing"
	"time"
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

func TestLLMToolkit_MonitorHardwareUsageForPID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	callCount := 0
	err := MonitorHardwareUsageForPID(ctx, int32(os.Getpid()), 300*time.Millisecond, func(info UsageInfo) error {
		if info.Process == nil {
			t.Fatal("expected process info")
		}
		callCount++
		cancel()
		return nil
	})
	if err != context.Canceled {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if callCount == 0 {
		t.Fatal("expected at least one usage callback")
	}
}
