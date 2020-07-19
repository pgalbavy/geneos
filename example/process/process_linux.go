// +build linux
package process

import (
	"wonderland.org/geneos/samplers"
)

func (p ProcessSampler) initColumns() (cols samplers.Columns, columnnames []string, sortcol string, err error) {
	return p.ColumnInfo(nil)
}
