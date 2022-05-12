package geneos

type optstruct struct {
	override     string
	local        bool
	nosave       bool
	overwrite    bool
	basename     string
	homedir      string
	version      string
	username     string
	password     string
	platform_id  string
	downloadbase string
	downloadtype string
	filename     string
}

type GeneosOptions func(*optstruct)

func doOptions(options ...GeneosOptions) (d *optstruct) {
	// defaults
	d = &optstruct{
		downloadbase: "releases",
		downloadtype: "resources",
	}
	for _, opt := range options {
		opt(d)
	}
	return
}

func NoSave(n bool) GeneosOptions {
	return func(d *optstruct) { d.nosave = n }
}

func LocalOnly(l bool) GeneosOptions {
	return func(d *optstruct) { d.local = l }
}

func Force(o bool) GeneosOptions {
	return func(d *optstruct) { d.overwrite = o }
}

func OverrideVersion(s string) GeneosOptions {
	return func(d *optstruct) { d.override = s }
}

func Version(v string) GeneosOptions {
	return func(d *optstruct) { d.version = v }
}

func Basename(b string) GeneosOptions {
	return func(d *optstruct) { d.basename = b }
}

func Homedir(h string) GeneosOptions {
	return func(d *optstruct) { d.homedir = h }
}

func Username(u string) GeneosOptions {
	return func(d *optstruct) { d.username = u }
}

func Password(p string) GeneosOptions {
	return func(d *optstruct) { d.password = p }
}

func PlatformID(id string) GeneosOptions {
	return func(d *optstruct) { d.platform_id = id }
}

func UseNexus() GeneosOptions {
	return func(d *optstruct) { d.downloadtype = "nexus" }
}

func UseSnapshots() GeneosOptions {
	return func(d *optstruct) { d.downloadbase = "snapshots" }
}

func Filename(f string) GeneosOptions {
	return func(d *optstruct) { d.filename = f }
}
