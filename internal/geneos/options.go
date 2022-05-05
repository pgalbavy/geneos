package geneos

type optstruct struct {
	override  string
	local     bool
	nosave    bool
	overwrite bool
	version   string
	basename  string
	homedir   string
	username  string
}

type GeneosOptions func(*optstruct)

func doOptions(options ...GeneosOptions) (d *optstruct) {
	d = &optstruct{}
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
