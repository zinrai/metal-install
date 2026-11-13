// Package data defines the shape of the data model.
package data

type Machine struct {
	BMCType       string         `yaml:"bmc_type"`
	NICs          []NIC          `yaml:"nics"`
	Bonds         []Bond         `yaml:"bonds"`
	UnusedNICs    []string       `yaml:"unused_nics"`
	KernelParams  []string       `yaml:"kernel_params"`
	ExtraPackages []ExtraPackage `yaml:"extra_packages"`
	Partition     Partition      `yaml:"partition"`
}

// Partition holds the disk layout once per installer, each in that
// installer's native syntax. Not a single neutral layout: that would
// make metal-install translate between installer formats.
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

type OS struct {
	Family       string    `yaml:"family"`
	Version      string    `yaml:"version"`
	BootImage    BootImage `yaml:"boot_image"`
	BootTemplate string    `yaml:"boot_template"`
	// A list rather than one file per OS: cloud-init's nocloud-net data
	// source needs user-data and meta-data side by side.
	Configs         []ConfigOutput `yaml:"configs"`
	Setup           Setup          `yaml:"setup"`
	Packages        []string       `yaml:"packages"`
	ExcludePackages []string       `yaml:"exclude_packages"`
	Timezone        string         `yaml:"timezone"`
	NTPServer       string         `yaml:"ntp_server"`
	Keyboard        string         `yaml:"keyboard"`
	Language        string         `yaml:"language"`
}

type ConfigOutput struct {
	Template string `yaml:"template"`
	Filename string `yaml:"filename"`
}

type Setup struct {
	Post []string `yaml:"post"`
}

type BootImage struct {
	Kernel string `yaml:"kernel"`
	Initrd string `yaml:"initrd"`
	Repo   string `yaml:"repo"`
}

type CompatEntry struct {
	Machines []string `yaml:"machines"`
	OS       []string `yaml:"os"`
}

type Env struct {
	InstallServer InstallServerEnv `yaml:"install_server"`
}

type InstallServerEnv struct {
	HTTPBase string `yaml:"http_base"`
}

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

type DataSet struct {
	Machines map[string]*Machine
	OS       map[string]*OS
	Compat   []CompatEntry
	Env      *Env

	// Keyed by path relative to the data directory. Holds content, not
	// paths: render does no filesystem access.
	Templates map[string]string
}
