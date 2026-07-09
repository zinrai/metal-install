// Package data defines the shape of the data model.
//
// The types here mirror the YAML files in machines/, os/, env.yml,
// compatibility.yml. They are loaded by package datadir and consumed
// by package render.
//
// Field naming convention: yaml tags are lowercase_with_underscores
// to match the file format. Go field names use idiomatic CamelCase.
package data

// Machine is what a single machines/<id>.yml file describes.
type Machine struct {
	BMCType       string         `yaml:"bmc_type"`
	NICs          []NIC          `yaml:"nics"`
	Bonds         []Bond         `yaml:"bonds"`
	UnusedNICs    []string       `yaml:"unused_nics"`
	KernelParams  []string       `yaml:"kernel_params"`
	ExtraPackages []ExtraPackage `yaml:"extra_packages"`
	Partition     Partition      `yaml:"partition"`
}

// Partition holds the install-disk and partition layout in each
// installer's native syntax. The same logical layout is expressed
// three times because metal-install does not translate between
// installer formats; each templated installer config substitutes the
// matching string directly.
//
// The premise is "one machine model = one disk layout". When the
// machine model is known, the disk layout is also known, so there is
// no need for runtime disk detection.
type Partition struct {
	Kickstart   string `yaml:"kickstart"`
	Preseed     string `yaml:"preseed"`
	Autoinstall string `yaml:"autoinstall"`
}

type NIC struct {
	PCI  string `yaml:"pci"`
	Name string `yaml:"name"`
}

type Bond struct {
	Name    string   `yaml:"name"`
	Members []string `yaml:"members"`
	Role    string   `yaml:"role"`
}

type ExtraPackage struct {
	Name        string `yaml:"name"`
	RPM         string `yaml:"rpm"`
	SymlinkFrom string `yaml:"symlink_from"`
	SymlinkTo   string `yaml:"symlink_to"`
}

// OS is what a single os/<id>.yml file describes.
type OS struct {
	Family          string         `yaml:"family"`
	Version         string         `yaml:"version"`
	BootImage       BootImage      `yaml:"boot_image"`
	BootTemplate    string         `yaml:"boot_template"`
	Configs         []ConfigOutput `yaml:"configs"`
	Setup           Setup          `yaml:"setup"`
	Packages        []string       `yaml:"packages"`
	ExcludePackages []string       `yaml:"exclude_packages"`
	Timezone        string         `yaml:"timezone"`
	NTPServer       string         `yaml:"ntp_server"`
	Keyboard        string         `yaml:"keyboard"`
	Language        string         `yaml:"language"`
}

// ConfigOutput is one installer-configuration file to render for an OS.
//
// RHEL family has one (kickstart.ks). Debian has one (preseed.cfg).
// Ubuntu autoinstall has two (user-data and meta-data), because the
// cloud-init nocloud-net data source requires both files to exist at
// the same URL.
type ConfigOutput struct {
	Template string `yaml:"template"`
	Filename string `yaml:"filename"`
}

// Setup lists the helper shell scripts that the installer fetches at
// different points in its lifecycle. Each entry is a template path
// (relative to the data root) whose rendered output is served at
// /configs/<node_id>/<phase>/<basename> where <basename> is the
// template filename minus the ".tmpl" suffix.
//
// Only `post` is supported today. `pre` is reserved for scripts that
// run before the installer reaches its main install phase; add it
// when there is a concrete need.
type Setup struct {
	Post []string `yaml:"post"`
}

type BootImage struct {
	Kernel string `yaml:"kernel"`
	Initrd string `yaml:"initrd"`
	Repo   string `yaml:"repo"`
}

// CompatEntry is one line of compatibility.yml.
type CompatEntry struct {
	Machines []string `yaml:"machines"`
	OS       []string `yaml:"os"`
}

// Env is env.yml: deployment-specific values.
type Env struct {
	InstallServer InstallServerEnv `yaml:"install_server"`
}

type InstallServerEnv struct {
	HTTPBase string `yaml:"http_base"`
}

// Spec is the InstallSpec: one installation instance.
//
// Both YAML and JSON tags are provided because metal-install-render
// reads YAML from disk while metal-install-server receives JSON over
// HTTP. The two formats are kept in sync by sharing this struct.
type Spec struct {
	Machine          string   `yaml:"machine" json:"machine"`
	OS               string   `yaml:"os" json:"os"`
	NodeID           string   `yaml:"node_id" json:"node_id"`
	IPv4Addr         string   `yaml:"ipv4_addr" json:"ipv4_addr"`
	PrefixLength     int      `yaml:"prefix_length" json:"prefix_length"`
	Gateway          string   `yaml:"gateway" json:"gateway"`
	DNS              string   `yaml:"dns" json:"dns"`
	RootPasswordHash string   `yaml:"root_password_hash" json:"root_password_hash"`
	SSHKeys          []string `yaml:"ssh_keys" json:"ssh_keys"`
}

// DataSet is the in-memory representation of an entire data
// directory: machines + os + compat + env + templates + scripts.
//
// Loaded once at startup by datadir.Load(). Render functions take a
// pointer to DataSet and look up referenced machines/os by ID.
type DataSet struct {
	Machines map[string]*Machine
	OS       map[string]*OS
	Compat   []CompatEntry
	Env      *Env

	// Templates is path -> content. Template content is loaded
	// upfront so that template.Parse can be called without further
	// filesystem access. Keys are paths relative to the data
	// directory (e.g. "templates/kickstart.tmpl").
	Templates map[string]string
}
