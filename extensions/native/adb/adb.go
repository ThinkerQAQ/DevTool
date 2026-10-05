package adb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/thinkerqaq/devtool/sdk/nativeadb"
	service "github.com/thinkerqaq/devtool/sdk/service"
	extensioncontract "github.com/thinkerqaq/devtool/sdk/extension"
)

const ExtensionID = "native.adb"

type Extension struct {
	executable string
	run        func(context.Context, ...string) ([]byte, error)
}

func New() *Extension {
	e := &Extension{executable: "adb"}
	e.run = e.runCommand
	return e
}

func (e *Extension) Descriptor() extensioncontract.Descriptor {
	return extensioncontract.Descriptor{
		ID:       ExtensionID,
		Kind:     extensioncontract.KindNative,
		Provides: []string{nativeadb.ServiceName},
	}
}

func (e *Extension) Configure(settings map[string]any) error {
	raw, ok := settings["executable"]
	if !ok {
		return nil
	}
	value, ok := raw.(string)
	if !ok {
		return fmt.Errorf("native.adb settings.executable must be a string")
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("native.adb settings.executable cannot be empty")
	}
	e.executable = value
	return nil
}

func (e *Extension) Register(reg extensioncontract.Registrar) error {
	return reg.ProvideService(nativeadb.ServiceName, ExtensionID, service.Func(e.Invoke))
}

func (e *Extension) Invoke(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	switch method {
	case nativeadb.MethodDoctor:
		return e.doctor(ctx)
	case nativeadb.MethodDevices:
		devices, err := e.devices(ctx)
		if err != nil {
			return nil, err
		}
		return json.Marshal(nativeadb.DevicesResponse{Provider: ExtensionID, Devices: devices})
	case nativeadb.MethodInstall:
		var request nativeadb.InstallRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode adb install request: %w", err)
		}
		return e.install(ctx, request)
	case nativeadb.MethodReverse:
		var request nativeadb.ReverseRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode adb reverse request: %w", err)
		}
		return e.reverse(ctx, request)
	case nativeadb.MethodLaunch:
		var request nativeadb.LaunchRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode adb launch request: %w", err)
		}
		return e.launch(ctx, request)
	case nativeadb.MethodShell:
		var request nativeadb.ShellRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode adb shell request: %w", err)
		}
		if len(request.Args) == 0 {
			return nil, fmt.Errorf("adb shell args are required")
		}
		return e.commandResponse(ctx, targetArgs(request.Serial, append([]string{"shell"}, request.Args...)...)...)
	case nativeadb.MethodLogcat:
		var request nativeadb.LogcatRequest
		if err := json.Unmarshal(payload, &request); err != nil {
			return nil, fmt.Errorf("decode adb logcat request: %w", err)
		}
		args := append([]string{"logcat", "-d"}, request.Args...)
		return e.commandResponse(ctx, targetArgs(request.Serial, args...)...)
	default:
		return nil, fmt.Errorf("%s does not support method %q", ExtensionID, method)
	}
}

func (e *Extension) doctor(ctx context.Context) (json.RawMessage, error) {
	version, err := e.execute(ctx, "version")
	if err != nil {
		return nil, fmt.Errorf("adb version: %w", err)
	}
	devices, err := e.devices(ctx)
	if err != nil {
		return nil, err
	}
	return json.Marshal(nativeadb.DoctorResponse{
		Provider:   ExtensionID,
		Executable: e.executable,
		Version:    strings.TrimSpace(string(version)),
		Devices:    devices,
	})
}

func (e *Extension) devices(ctx context.Context) ([]nativeadb.Device, error) {
	out, err := e.execute(ctx, "devices", "-l")
	if err != nil {
		return nil, fmt.Errorf("adb devices: %w", err)
	}
	return parseDevices(string(out)), nil
}

func (e *Extension) install(ctx context.Context, request nativeadb.InstallRequest) (json.RawMessage, error) {
	apk := strings.TrimSpace(request.APK)
	if apk == "" {
		return nil, fmt.Errorf("adb install apk is required")
	}
	absolute, err := filepath.Abs(apk)
	if err != nil {
		return nil, fmt.Errorf("resolve apk path: %w", err)
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return nil, fmt.Errorf("adb install apk %s: %w", absolute, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("adb install apk %s is a directory", absolute)
	}
	args := []string{"install"}
	if request.Replace {
		args = append(args, "-r")
	}
	args = append(args, absolute)
	return e.commandResponse(ctx, targetArgs(request.Serial, args...)...)
}

func (e *Extension) reverse(ctx context.Context, request nativeadb.ReverseRequest) (json.RawMessage, error) {
	if err := validatePort("device_port", request.DevicePort); err != nil {
		return nil, err
	}
	if err := validatePort("host_port", request.HostPort); err != nil {
		return nil, err
	}
	return e.commandResponse(ctx, targetArgs(request.Serial,
		"reverse",
		fmt.Sprintf("tcp:%d", request.DevicePort),
		fmt.Sprintf("tcp:%d", request.HostPort),
	)...)
}

func (e *Extension) launch(ctx context.Context, request nativeadb.LaunchRequest) (json.RawMessage, error) {
	pkg := strings.TrimSpace(request.Package)
	if pkg == "" {
		return nil, fmt.Errorf("adb launch package is required")
	}
	activity := strings.TrimSpace(request.Activity)
	if activity != "" {
		return e.commandResponse(ctx, targetArgs(request.Serial, "shell", "am", "start", "-n", pkg+"/"+activity)...)
	}
	return e.commandResponse(ctx, targetArgs(request.Serial,
		"shell", "monkey", "-p", pkg, "-c", "android.intent.category.LAUNCHER", "1",
	)...)
}

func (e *Extension) commandResponse(ctx context.Context, args ...string) (json.RawMessage, error) {
	out, err := e.execute(ctx, args...)
	if err != nil {
		return nil, err
	}
	return json.Marshal(nativeadb.CommandResponse{Provider: ExtensionID, Output: strings.TrimSpace(string(out))})
}

func (e *Extension) execute(ctx context.Context, args ...string) ([]byte, error) {
	if e.run == nil {
		return nil, fmt.Errorf("adb command runner is unavailable")
	}
	out, err := e.run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w: %s", e.executable, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func (e *Extension) runCommand(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, e.executable, args...).CombinedOutput()
}

func targetArgs(serial string, args ...string) []string {
	out := make([]string, 0, len(args)+2)
	if serial = strings.TrimSpace(serial); serial != "" {
		out = append(out, "-s", serial)
	}
	return append(out, args...)
}

func validatePort(name string, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s must be between 1 and 65535", name)
	}
	return nil
}

func parseDevices(raw string) []nativeadb.Device {
	var devices []nativeadb.Device
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of devices attached") || strings.HasPrefix(line, "* daemon") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		device := nativeadb.Device{Serial: fields[0], State: fields[1]}
		if len(fields) > 2 {
			device.Details = strings.Join(fields[2:], " ")
		}
		devices = append(devices, device)
	}
	return devices
}
