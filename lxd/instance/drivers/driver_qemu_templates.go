package drivers

import (
	"fmt"
	"strings"
	"text/template"
)

type cfgEntry struct {
	key   string
	value string
}

type cfgSection struct {
	name    string
	comment string
	entries []cfgEntry
}

func qemuAppendSections(sb *strings.Builder, sections ...cfgSection) {
	for _, section := range sections {
		if section.comment != "" {
			sb.WriteString(fmt.Sprintf("# %s\n", section.comment))
		}

		sb.WriteString(fmt.Sprintf("[%s]\n", section.name))

		for _, entry := range section.entries {
			value := entry.value
			if value != "" {
				sb.WriteString(fmt.Sprintf("%s = \"%s\"\n", entry.key, value))
			}
		}

		sb.WriteString("\n")
	}
}

func qemuBaseSections(architecture string) []cfgSection {
	machineType := ""
	gicVersion := ""
	capLargeDecr := ""

	switch architecture {
	case "x86_64":
		machineType = "q35"
	case "aarch64":
		machineType = "virt"
		gicVersion = "max"
	case "ppc64le":
		machineType = "pseries"
		capLargeDecr = "off"
	case "s390x":
		machineType = "s390-ccw-virtio"
	}

	sections := []cfgSection{{
		name:    "machine",
		comment: "Machine",
		entries: []cfgEntry{
			{key: "graphics", value: "off"},
			{key: "type", value: machineType},
			{key: "gic-version", value: gicVersion},
			{key: "cap-large-decr", value: capLargeDecr},
			{key: "accel", value: "kvm"},
			{key: "usb", value: "off"},
		},
	}}

	if architecture == "x86_64" {
		sections = append(sections, []cfgSection{{
			name: "global",
			entries: []cfgEntry{
				{key: "driver", value: "ICH9-LPC"},
				{key: "property", value: "disable_s3"},
				{key: "value", value: "1"},
			},
		}, {
			name: "global",
			entries: []cfgEntry{
				{key: "driver", value: "ICH9-LPC"},
				{key: "property", value: "disable_s4"},
				{key: "value", value: "1"},
			},
		}}...)
	}

	return append(
		sections,
		cfgSection{
			name:    "boot-opts",
			entries: []cfgEntry{{key: "strict", value: "on"}},
		})
}

type qemuMemoryOpts struct {
	memSizeMB int64
}

func qemuMemorySections(opts *qemuMemoryOpts) []cfgSection {
	return []cfgSection{{
		name:    "memory",
		comment: "Memory",
		entries: []cfgEntry{{key: "size", value: fmt.Sprintf("%dM", opts.memSizeMB)}},
	}}
}

type qemuDevOpts struct {
	busName       string
	devBus        string
	devAddr       string
	multifunction bool
}

type qemuDevEntriesOpts struct {
	dev     qemuDevOpts
	pciName string
	ccwName string
}

func qemuDeviceEntries(opts *qemuDevEntriesOpts) []cfgEntry {
	entries := []cfgEntry{}

	if opts.dev.busName == "pci" || opts.dev.busName == "pcie" {
		entries = append(entries, []cfgEntry{
			{key: "driver", value: opts.pciName},
			{key: "bus", value: opts.dev.devBus},
			{key: "addr", value: opts.dev.devAddr},
		}...)
	} else if opts.dev.busName == "ccw" {
		entries = append(entries, cfgEntry{key: "driver", value: opts.ccwName})
	}

	if opts.dev.multifunction {
		entries = append(entries, cfgEntry{key: "multifunction", value: "on"})
	}

	return entries
}

type qemuSerialOpts struct {
	dev              qemuDevOpts
	charDevName      string
	ringbufSizeBytes int
}

func qemuSerialSections(opts *qemuSerialOpts) []cfgSection {
	entriesOpts := qemuDevEntriesOpts{
		dev:     opts.dev,
		pciName: "virtio-serial-pci",
		ccwName: "virtio-serial-ccw",
	}

	return []cfgSection{{
		name:    `device "dev-qemu_serial"`,
		comment: "Virtual serial bus",
		entries: qemuDeviceEntries(&entriesOpts),
	}, {
		name:    fmt.Sprintf(`chardev "%s"`, opts.charDevName),
		comment: "LXD serial identifier",
		entries: []cfgEntry{
			{key: "backend", value: "ringbuf"},
			{key: "size", value: fmt.Sprintf("%dB", opts.ringbufSizeBytes)}},
	}, {
		name: `device "qemu_serial"`,
		entries: []cfgEntry{
			{key: "driver", value: "virtserialport"},
			{key: "name", value: "org.linuxcontainers.lxd"},
			{key: "chardev", value: opts.charDevName},
			{key: "bus", value: "dev-qemu_serial.0"},
		},
	}, {
		name:    `chardev "qemu_spice-chardev"`,
		comment: "Spice agent",
		entries: []cfgEntry{
			{key: "backend", value: "spicevmc"},
			{key: "name", value: "vdagent"},
		},
	}, {
		name: `device "qemu_spice"`,
		entries: []cfgEntry{
			{key: "driver", value: "virtserialport"},
			{key: "name", value: "com.redhat.spice.0"},
			{key: "chardev", value: "qemu_spice-chardev"},
			{key: "bus", value: "dev-qemu_serial.0"},
		},
	}, {
		name:    `chardev "qemu_spicedir-chardev"`,
		comment: "Spice folder",
		entries: []cfgEntry{
			{key: "backend", value: "spiceport"},
			{key: "name", value: "org.spice-space.webdav.0"},
		},
	}, {
		name: `device "qemu_spicedir"`,
		entries: []cfgEntry{
			{key: "driver", value: "virtserialport"},
			{key: "name", value: "org.spice-space.webdav.0"},
			{key: "chardev", value: "qemu_spicedir-chardev"},
			{key: "bus", value: "dev-qemu_serial.0"},
		},
	}}
}

var qemuSerial = template.Must(template.New("qemuSerial").Parse(`
# Virtual serial bus
[device "dev-qemu_serial"]
{{- if eq .bus "pci" "pcie"}}
driver = "virtio-serial-pci"
bus = "{{.devBus}}"
addr = "{{.devAddr}}"
{{- end}}
{{if eq .bus "ccw" -}}
driver = "virtio-serial-ccw"
{{- end}}
{{if .multifunction -}}
multifunction = "on"
{{- end }}

# LXD serial identifier
[chardev "{{.chardevName}}"]
backend = "ringbuf"
size = "{{.ringbufSizeBytes}}B"

[device "qemu_serial"]
driver = "virtserialport"
name = "org.linuxcontainers.lxd"
chardev = "{{.chardevName}}"
bus = "dev-qemu_serial.0"

# Spice agent
[chardev "qemu_spice-chardev"]
backend = "spicevmc"
name = "vdagent"

[device "qemu_spice"]
driver = "virtserialport"
name = "com.redhat.spice.0"
chardev = "qemu_spice-chardev"
bus = "dev-qemu_serial.0"

# Spice folder
[chardev "qemu_spicedir-chardev"]
backend = "spiceport"
name = "org.spice-space.webdav.0"

[device "qemu_spicedir"]
driver = "virtserialport"
name = "org.spice-space.webdav.0"
chardev = "qemu_spicedir-chardev"
bus = "dev-qemu_serial.0"
`))

var qemuPCIe = template.Must(template.New("qemuPCIe").Parse(`
[device "{{.portName}}"]
driver = "pcie-root-port"
bus = "pcie.0"
addr = "{{.addr}}"
chassis = "{{.index}}"
{{if .multifunction -}}
multifunction = "on"
{{- end }}
`))

var qemuSCSI = template.Must(template.New("qemuSCSI").Parse(`
# SCSI controller
[device "qemu_scsi"]
{{- if eq .bus "pci" "pcie"}}
driver = "virtio-scsi-pci"
bus = "{{.devBus}}"
addr = "{{.devAddr}}"
{{- end}}
{{if eq .bus "ccw" -}}
driver = "virtio-scsi-ccw"
{{- end}}
{{if .multifunction -}}
multifunction = "on"
{{- end }}
`))

var qemuBalloon = template.Must(template.New("qemuBalloon").Parse(`
# Balloon driver
[device "qemu_balloon"]
{{- if eq .bus "pci" "pcie"}}
driver = "virtio-balloon-pci"
bus = "{{.devBus}}"
addr = "{{.devAddr}}"
{{- end}}
{{if eq .bus "ccw" -}}
driver = "virtio-balloon-ccw"
{{- end}}
{{if .multifunction -}}
multifunction = "on"
{{- end }}
`))

var qemuRNG = template.Must(template.New("qemuRNG").Parse(`
# Random number generator
[object "qemu_rng"]
qom-type = "rng-random"
filename = "/dev/urandom"

[device "dev-qemu_rng"]
{{- if eq .bus "pci" "pcie"}}
driver = "virtio-rng-pci"
bus = "{{.devBus}}"
addr = "{{.devAddr}}"
{{- end}}
{{if eq .bus "ccw" -}}
driver = "virtio-rng-ccw"
{{- end}}
rng = "qemu_rng"
{{if .multifunction -}}
multifunction = "on"
{{- end }}
`))

var qemuVsock = template.Must(template.New("qemuVsock").Parse(`
# Vsock
[device "qemu_vsock"]
{{- if eq .bus "pci" "pcie"}}
driver = "vhost-vsock-pci"
bus = "{{.devBus}}"
addr = "{{.devAddr}}"
{{- end}}
{{if eq .bus "ccw" -}}
driver = "vhost-vsock-ccw"
{{- end}}
guest-cid = "{{.vsockID}}"
{{if .multifunction -}}
multifunction = "on"
{{- end }}
`))

var qemuGPU = template.Must(template.New("qemuGPU").Parse(`
# GPU
[device "qemu_gpu"]
{{- if eq .bus "pci" "pcie"}}
{{if eq .architecture "x86_64" -}}
driver = "virtio-vga"
{{- else}}
driver = "virtio-gpu-pci"
{{- end}}
bus = "{{.devBus}}"
addr = "{{.devAddr}}"
{{- end}}
{{if eq .bus "ccw" -}}
driver = "virtio-gpu-ccw"
{{- end}}
{{if .multifunction -}}
multifunction = "on"
{{- end }}
`))

var qemuKeyboard = template.Must(template.New("qemuKeyboard").Parse(`
# Input
[device "qemu_keyboard"]
{{- if eq .bus "pci" "pcie"}}
driver = "virtio-keyboard-pci"
bus = "{{.devBus}}"
addr = "{{.devAddr}}"
{{- end}}
{{if eq .bus "ccw" -}}
driver = "virtio-keyboard-ccw"
{{- end}}
{{if .multifunction -}}
multifunction = "on"
{{- end }}
`))

var qemuTablet = template.Must(template.New("qemuTablet").Parse(`
# Input
[device "qemu_tablet"]
{{- if eq .bus "pci" "pcie"}}
driver = "virtio-tablet-pci"
bus = "{{.devBus}}"
addr = "{{.devAddr}}"
{{- end}}
{{if eq .bus "ccw" -}}
driver = "virtio-tablet-ccw"
{{- end}}
{{if .multifunction -}}
multifunction = "on"
{{- end }}
`))

var qemuCPU = template.Must(template.New("qemuCPU").Parse(`
# CPU
[smp-opts]
cpus = "{{.cpuCount}}"
sockets = "{{.cpuSockets}}"
cores = "{{.cpuCores}}"
threads = "{{.cpuThreads}}"

{{if eq .architecture "x86_64" -}}
{{$memory := .memory -}}
{{$hugepages := .hugepages -}}
{{if .cpuNumaHostNodes -}}
{{range $index, $element := .cpuNumaHostNodes}}
[object "mem{{$index}}"]
{{if ne $hugepages "" -}}
qom-type = "memory-backend-file"
mem-path = "{{$hugepages}}"
prealloc = "on"
discard-data = "on"
share = "on"
{{- else}}
qom-type = "memory-backend-memfd"
{{- end }}
size = "{{$memory}}M"
policy = "bind"
{{- if eq $.qemuMemObjectFormat "indexed"}}
host-nodes.0 = "{{$element}}"
{{- else}}
host-nodes = "{{$element}}"
{{- end}}

[numa]
type = "node"
nodeid = "{{$index}}"
memdev = "mem{{$index}}"
{{end}}
{{else}}
[object "mem0"]
{{if ne $hugepages "" -}}
qom-type = "memory-backend-file"
mem-path = "{{$hugepages}}"
prealloc = "on"
discard-data = "on"
{{- else}}
qom-type = "memory-backend-memfd"
{{- end }}
size = "{{$memory}}M"
share = "on"

[numa]
type = "node"
nodeid = "0"
memdev = "mem0"
{{end}}

{{range .cpuNumaMapping}}
[numa]
type = "cpu"
node-id = "{{.node}}"
socket-id = "{{.socket}}"
core-id = "{{.core}}"
thread-id = "{{.thread}}"
{{end}}
{{end}}
`))

var qemuControlSocket = template.Must(template.New("qemuControlSocket").Parse(`
# Qemu control
[chardev "monitor"]
backend = "socket"
path = "{{.path}}"
server = "on"
wait = "off"

[mon]
chardev = "monitor"
mode = "control"
`))

var qemuConsole = template.Must(template.New("qemuConsole").Parse(`
# Console
[chardev "console"]
backend = "socket"
path = "{{.path}}"
server = "on"
wait = "off"
`))

var qemuDriveFirmware = template.Must(template.New("qemuDriveFirmware").Parse(`
# Firmware (read only)
[drive]
file = "{{.roPath}}"
if = "pflash"
format = "raw"
unit = "0"
readonly = "on"

# Firmware settings (writable)
[drive]
file = "{{.nvramPath}}"
if = "pflash"
format = "raw"
unit = "1"
`))

// Devices use "qemu_" prefix indicating that this is a internally named device.
var qemuDriveConfig = template.Must(template.New("qemuDriveConfig").Parse(`
# Config drive ({{.protocol}})
{{- if eq .protocol "9p" }}
[fsdev "qemu_config"]
fsdriver = "local"
security_model = "none"
readonly = "on"
path = "{{.path}}"
{{- else if eq .protocol "virtio-fs" }}
[chardev "qemu_config"]
backend = "socket"
path = "{{.path}}"
{{- end }}

[device "dev-qemu_config-drive-{{.protocol}}"]
{{- if eq .bus "pci" "pcie"}}
{{- if eq .protocol "9p" }}
driver = "virtio-9p-pci"
{{- else if eq .protocol "virtio-fs" }}
driver = "vhost-user-fs-pci"
{{- end }}
bus = "{{.devBus}}"
addr = "{{.devAddr}}"
{{- end}}
{{- if eq .bus "ccw" }}
{{- if eq .protocol "9p" }}
driver = "virtio-9p-ccw"
{{- else if eq .protocol "virtio-fs" }}
driver = "vhost-user-fs-ccw"
{{- end }}
{{- end}}
{{- if eq .protocol "9p" }}
mount_tag = "config"
fsdev = "qemu_config"
{{- else if eq .protocol "virtio-fs" }}
chardev = "qemu_config"
tag = "config"
{{- end }}
{{if .multifunction -}}
multifunction = "on"
{{- end }}
`))

// Devices use "lxd_" prefix indicating that this is a user named device.
var qemuDriveDir = template.Must(template.New("qemuDriveDir").Parse(`
# {{.devName}} drive ({{.protocol}})
{{- if eq .protocol "9p" }}
[fsdev "lxd_{{.devName}}"]
fsdriver = "proxy"
sock_fd = "{{.proxyFD}}"
{{- if .readonly}}
readonly = "on"
{{- else}}
readonly = "off"
{{- end}}
{{- else if eq .protocol "virtio-fs" }}
[chardev "lxd_{{.devName}}"]
backend = "socket"
path = "{{.path}}"
{{- end }}

[device "dev-lxd_{{.devName}}-{{.protocol}}"]
{{- if eq .bus "pci" "pcie"}}
{{- if eq .protocol "9p" }}
driver = "virtio-9p-pci"
{{- else if eq .protocol "virtio-fs" }}
driver = "vhost-user-fs-pci"
{{- end }}
bus = "{{.devBus}}"
addr = "{{.devAddr}}"
{{- end -}}
{{if eq .bus "ccw" -}}
{{- if eq .protocol "9p" }}
driver = "virtio-9p-ccw"
{{- else if eq .protocol "virtio-fs" }}
driver = "vhost-user-fs-ccw"
{{- end }}
{{- end}}
{{- if eq .protocol "9p" }}
fsdev = "lxd_{{.devName}}"
mount_tag = "{{.mountTag}}"
{{- else if eq .protocol "virtio-fs" }}
chardev = "lxd_{{.devName}}"
tag = "{{.mountTag}}"
{{- end }}
{{if .multifunction -}}
multifunction = "on"
{{- end }}
`))

// Devices use "lxd_" prefix indicating that this is a user named device.
var qemuPCIPhysical = template.Must(template.New("qemuPCIPhysical").Parse(`
# PCI card ("{{.devName}}" device)
[device "dev-lxd_{{.devName}}"]
{{- if eq .bus "pci" "pcie"}}
driver = "vfio-pci"
bus = "{{.devBus}}"
addr = "{{.devAddr}}"
{{- end}}
{{if eq .bus "ccw" -}}
driver = "vfio-ccw"
{{- end}}
host = "{{.pciSlotName}}"
{{if .bootIndex -}}
bootindex = "{{.bootIndex}}"
{{- end }}
{{if .multifunction -}}
multifunction = "on"
{{- end }}
`))

// Devices use "lxd_" prefix indicating that this is a user named device.
var qemuGPUDevPhysical = template.Must(template.New("qemuGPUDevPhysical").Parse(`
# GPU card ("{{.devName}}" device)
[device "dev-lxd_{{.devName}}"]
{{- if eq .bus "pci" "pcie"}}
driver = "vfio-pci"
bus = "{{.devBus}}"
addr = "{{.devAddr}}"
{{- end}}
{{if eq .bus "ccw" -}}
driver = "vfio-ccw"
{{- end}}
{{- if ne .vgpu "" -}}
sysfsdev = "/sys/bus/mdev/devices/{{.vgpu}}"
{{- else}}
host = "{{.pciSlotName}}"
{{if .vga -}}
x-vga = "on"
{{- end }}
{{- end }}
{{if .multifunction -}}
multifunction = "on"
{{- end }}
`))

var qemuUSB = template.Must(template.New("qemuUSB").Parse(`
# USB controller
[device "qemu_usb"]
driver = "qemu-xhci"
bus = "{{.devBus}}"
addr = "{{.devAddr}}"
p2 = "{{.ports}}"
p3 = "{{.ports}}"
{{if .multifunction -}}
multifunction = "on"
{{- end }}

[chardev "qemu_spice-usb-chardev1"]
  backend = "spicevmc"
  name = "usbredir"

[chardev "qemu_spice-usb-chardev2"]
  backend = "spicevmc"
  name = "usbredir"

[chardev "qemu_spice-usb-chardev3"]
  backend = "spicevmc"
  name = "usbredir"

[device "qemu_spice-usb1"]
  driver = "usb-redir"
  chardev = "qemu_spice-usb-chardev1"

[device "qemu_spice-usb2"]
  driver = "usb-redir"
  chardev = "qemu_spice-usb-chardev2"

[device "qemu_spice-usb3"]
  driver = "usb-redir"
  chardev = "qemu_spice-usb-chardev3"
`))

var qemuTPM = template.Must(template.New("qemuTPM").Parse(`
[chardev "qemu_tpm-chardev_{{.devName}}"]
backend = "socket"
path = "{{.path}}"

[tpmdev "qemu_tpm-tpmdev_{{.devName}}"]
type = "emulator"
chardev = "qemu_tpm-chardev_{{.devName}}"

[device "dev-lxd_{{.devName}}"]
driver = "tpm-crb"
tpmdev = "qemu_tpm-tpmdev_{{.devName}}"
`))
