package nativeadb

const ServiceName = "adb"

const (
	MethodDoctor  = "doctor"
	MethodDevices = "devices"
	MethodInstall = "install"
	MethodReverse = "reverse"
	MethodLaunch  = "launch"
	MethodShell   = "shell"
	MethodLogcat  = "logcat"
)

type Device struct {
	Serial  string `json:"serial"`
	State   string `json:"state"`
	Details string `json:"details,omitempty"`
}

type DoctorRequest struct{}

type DoctorResponse struct {
	Provider   string   `json:"provider"`
	Executable string   `json:"executable"`
	Version    string   `json:"version,omitempty"`
	Devices    []Device `json:"devices,omitempty"`
}

type DevicesRequest struct{}

type DevicesResponse struct {
	Provider string   `json:"provider"`
	Devices  []Device `json:"devices"`
}

type InstallRequest struct {
	Serial  string `json:"serial,omitempty"`
	APK     string `json:"apk"`
	Replace bool   `json:"replace,omitempty"`
}

type ReverseRequest struct {
	Serial     string `json:"serial,omitempty"`
	DevicePort int    `json:"device_port"`
	HostPort   int    `json:"host_port"`
}

type LaunchRequest struct {
	Serial   string `json:"serial,omitempty"`
	Package  string `json:"package"`
	Activity string `json:"activity,omitempty"`
}

type ShellRequest struct {
	Serial string   `json:"serial,omitempty"`
	Args   []string `json:"args"`
}

type LogcatRequest struct {
	Serial string   `json:"serial,omitempty"`
	Args   []string `json:"args,omitempty"`
}

type CommandResponse struct {
	Provider string `json:"provider"`
	Output   string `json:"output,omitempty"`
}
