package adb

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/thinkerqaq/devtool/sdk/nativeadb"
)

func TestDevicesParsesADBOutput(t *testing.T) {
	devices := parseDevices("List of devices attached\nserial-1\tdevice product:foo model:bar\nserial-2\toffline\n")
	want := []nativeadb.Device{
		{Serial: "serial-1", State: "device", Details: "product:foo model:bar"},
		{Serial: "serial-2", State: "offline"},
	}
	if !reflect.DeepEqual(devices, want) {
		t.Fatalf("devices = %#v, want %#v", devices, want)
	}
}

func TestReverseUsesSelectedSerial(t *testing.T) {
	ext := New()
	var got []string
	ext.run = func(_ context.Context, args ...string) ([]byte, error) {
		got = append([]string(nil), args...)
		return []byte("ok"), nil
	}

	raw, err := json.Marshal(nativeadb.ReverseRequest{Serial: "abc", DevicePort: 8080, HostPort: 18080})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ext.Invoke(context.Background(), nativeadb.MethodReverse, raw); err != nil {
		t.Fatal(err)
	}
	want := []string{"-s", "abc", "reverse", "tcp:8080", "tcp:18080"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
}

func TestDoctorReportsUnavailableADB(t *testing.T) {
	ext := New()
	ext.run = func(_ context.Context, args ...string) ([]byte, error) {
		return []byte("missing"), errors.New("not found")
	}
	if _, err := ext.Invoke(context.Background(), nativeadb.MethodDoctor, json.RawMessage(`{}`)); err == nil {
		t.Fatal("doctor expected error")
	}
}
