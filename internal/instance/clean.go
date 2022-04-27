package instance

import (
	"os"

	"github.com/spf13/viper"
	"wonderland.org/geneos/internal/geneos"
)

func Clean(c geneos.Instance, purge bool, params []string) (err error) {
	var stopped bool

	cleanlist := viper.GetString(c.Type().CleanList)
	purgelist := viper.GetString(c.Type().PurgeList)

	if !purge {
		if cleanlist != "" {
			if err = RemovePaths(c, cleanlist); err == nil {
				logDebug.Println(c, "cleaned")
			}
		}
		return
	}

	if _, err = GetPID(c); err == os.ErrProcessDone {
		stopped = false
	} else if err = Stop(c, false); err != nil {
		return
	} else {
		stopped = true
	}

	if cleanlist != "" {
		if err = RemovePaths(c, cleanlist); err != nil {
			return
		}
	}
	if purgelist != "" {
		if err = RemovePaths(c, purgelist); err != nil {
			return
		}
	}
	logDebug.Println(c, "fully cleaned")
	if stopped {
		err = Start(c)
	}
	return
}
