package main

func init() {
	RegsiterCommand(Command{
		Name:          "reload",
		Function:      commandReload,
		ParseFlags:    defaultFlag,
		ParseArgs:     defaultArgs,
		Wildcard:      true,
		ComponentOnly: false,

		CommandLine: "geneos reload [TYPE] [NAME...]",
		Summary:     `Signal the instance to reload it's configuration, if supported.`,
		Description: `Signal the matching instances to reload their configurations, depending on the component TYPE.`,
	})
}

func commandReload(ct Component, args []string, params []string) error {
	return ct.loopCommand(reloadInstance, args, params)
}

func reloadInstance(c Instances, params []string) (err error) {
	return c.Reload(params)
}
