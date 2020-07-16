/*
Package xmlrpc implements a Golang API to the Geneos XML-RPC API.

All but one existing API call is implemented using a direct name conversion
from the API docs to Golang conforming function names. Parameters are passed
in the same order as the documented XML-RPC calls but moving the elements to
their own arguments, e.g.

string entity.sampler.view.addTableRow(string rowName)

becomes

AddTableRow(entity string, sampler string, view string, rowname string) error

Here, Golang error type is used instead of returning a string type as the only
valid return is "OK", which is treated as error = nil

Note that where is it required, group and view have been split into separate arguments and
are passed to the API in the correct format for the call.
*/
package xmlrpc // import "wonderland.org/geneos/xmlrpc"

import (
	"strconv"
	"strings"
)

/*
All methods have value receivers. This is intentional as none of the calls mutate the type
*/

// requires split view and group names
func (geneos Client) createView(entity string, sampler string, view string, group string) (err error) {
	method := strings.Join([]string{entity, sampler, "createView"}, ".")
	args := []valueArray{{String: view}, {String: group}}

	return geneos.methodWithArgs(method, args)
}

func (geneos Client) viewExists(entity string, sampler string, view string) (bool, error) {
	method := strings.Join([]string{entity, sampler, "viewExists"}, ".")
	args := []valueArray{{String: view}}

	return geneos.methodBoolWithArgs(method, args)
}

// requires split view and group names
func (geneos Client) removeView(entity string, sampler string, view string, group string) error {
	method := strings.Join([]string{entity, sampler, "removeView"}, ".")
	args := []valueArray{{String: view}, {String: group}}

	return geneos.methodWithArgs(method, args)
}

func (geneos Client) getParameter(entity string, sampler string, parameter string) (string, error) {
	method := strings.Join([]string{entity, sampler, "getParameter"}, ".")
	args := []valueArray{{String: parameter}}

	return geneos.methodStringWithArgs(method, args)
}

func (geneos Client) addTableRow(entity string, sampler string, view string, rowname string) error {
	method := strings.Join([]string{entity, sampler, view, "addTableRow"}, ".")
	args := []valueArray{{String: rowname}}

	return geneos.methodWithArgs(method, args)
}

func (geneos Client) addTableColumn(entity string, sampler string, view string, column string) error {
	method := strings.Join([]string{entity, sampler, view, "addTableColumn"}, ".")
	args := []valueArray{{String: column}}

	return geneos.methodWithArgs(method, args)
}

func (geneos Client) removeTableRow(entity string, sampler string, view string, rowname string) error {
	method := strings.Join([]string{entity, sampler, view, "removeTableRow"}, ".")
	args := []valueArray{{String: rowname}}

	return geneos.methodWithArgs(method, args)
}

func (geneos Client) addHeadline(entity string, sampler string, view string, headlinename string) error {
	method := strings.Join([]string{entity, sampler, view, "addHeadline"}, ".")
	args := []valueArray{{String: headlinename}}

	return geneos.methodWithArgs(method, args)
}

func (geneos Client) removeHeadline(entity string, sampler string, view string, rowname string) error {
	method := strings.Join([]string{entity, sampler, view, "removeHeadline"}, ".")
	args := []valueArray{{String: rowname}}

	return geneos.methodWithArgs(method, args)
}

func (geneos Client) updateVariable(entity string, sampler string, view string, variable string, value string) error {
	method := strings.Join([]string{entity, sampler, view, "updateVariable"}, ".")
	args := []valueArray{{String: variable}, {String: value}}

	return geneos.methodWithArgs(method, args)
}

func (geneos Client) updateHeadline(entity string, sampler string, view string, headline string, value string) error {
	method := strings.Join([]string{entity, sampler, view, "updateHeadline"}, ".")
	args := []valueArray{{String: headline}, {String: value}}

	return geneos.methodWithArgs(method, args)
}

func (geneos Client) updateTableCell(entity string, sampler string, view string, cellname string, value string) error {
	method := strings.Join([]string{entity, sampler, view, "updateTableCell"}, ".")
	args := []valueArray{{String: cellname}, {String: value}}

	return geneos.methodWithArgs(method, args)
}

func (geneos Client) updateTableRow(entity string, sampler string, view string, rowname string, values []string) error {
	method := strings.Join([]string{entity, sampler, view, "updateTableRow"}, ".")
	args := []valueArray{{String: rowname}, {Array: values}}

	return geneos.methodWithArgs(method, args)
}

func (geneos Client) updateEntireTable(entity string, sampler string, view string, values [][]string) error {
	method := strings.Join([]string{entity, sampler, view, "updateEntireTable"}, ".")
	args := []valueArray{{Array: values}}

	return geneos.methodWithArgs(method, args)
}

func (geneos Client) columnExists(entity string, sampler string, view string, column string) (bool, error) {
	method := strings.Join([]string{entity, sampler, view, "columnExists"}, ".")
	args := []valueArray{{String: column}}

	return geneos.methodBoolWithArgs(method, args)
}

func (geneos Client) rowExists(entity string, sampler string, view string, row string) (bool, error) {
	method := strings.Join([]string{entity, sampler, view, "rowExists"}, ".")
	args := []valueArray{{String: row}}

	return geneos.methodBoolWithArgs(method, args)
}

func (geneos Client) headlineExists(entity string, sampler string, view string, headline string) (bool, error) {
	method := strings.Join([]string{entity, sampler, view, "headlineExists"}, ".")
	args := []valueArray{{String: headline}}

	return geneos.methodBoolWithArgs(method, args)
}

func (geneos Client) getColumnCount(entity string, sampler string, view string) (int, error) {
	method := strings.Join([]string{entity, sampler, view, "getColumnCount"}, ".")

	return geneos.methodIntNoArgs(method)
}

func (geneos Client) getRowCount(entity string, sampler string, view string) (int, error) {
	method := strings.Join([]string{entity, sampler, view, "getRowCount"}, ".")

	return geneos.methodIntNoArgs(method)
}

func (geneos Client) getHeadlineCount(entity string, sampler string, view string) (int, error) {
	method := strings.Join([]string{entity, sampler, view, "getHeadlineCount"}, ".")

	return geneos.methodIntNoArgs(method)
}

func (geneos Client) getColumnNames(entity string, sampler string, view string) ([]string, error) {
	method := strings.Join([]string{entity, sampler, view, "getColumnNames"}, ".")

	return geneos.methodStringsNoArgs(method)
}

func (geneos Client) getRowNames(entity string, sampler string, view string) ([]string, error) {
	method := strings.Join([]string{entity, sampler, view, "getRowNames"}, ".")

	return geneos.methodStringsNoArgs(method)
}

func (geneos Client) getHeadlineNames(entity string, sampler string, view string) ([]string, error) {
	method := strings.Join([]string{entity, sampler, view, "getHeadlineNames"}, ".")

	return geneos.methodStringsNoArgs(method)
}

func (geneos Client) getRowNamesOlderThan(entity string, sampler string, view string, unixtime int64) ([]string, error) {
	method := strings.Join([]string{entity, sampler, view, "getRowNamesOlderThan"}, ".")
	args := []valueArray{valueArray{String: strconv.FormatInt(unixtime, 10)}}

	return geneos.methodStringsWithArgs(method, args)
}

func (geneos Client) signOn(entity string, sampler string, seconds int) (err error) {
	method := strings.Join([]string{entity, sampler, "signOn"}, ".")
	args := []valueArray{valueArray{Int: seconds}}

	return geneos.methodWithArgs(method, args)
}

func (geneos Client) signOff(entity string, sampler string) (err error) {
	method := strings.Join([]string{entity, sampler, "signOn"}, ".")

	return geneos.methodNoArgs(method)
}

func (geneos Client) heartbeat(entity string, sampler string) (err error) {
	method := strings.Join([]string{entity, sampler, "heartBeat"}, ".")

	return geneos.methodNoArgs(method)
}

func (geneos Client) addMessageStream(entity string, sampler string, stream string, message string) error {
	method := strings.Join([]string{entity, sampler, stream, "addMessage"}, ".")
	args := []valueArray{{String: message}}

	return geneos.methodWithArgs(method, args)
}

func (geneos Client) signOnStream(entity string, sampler string, stream string, seconds int) (err error) {
	method := strings.Join([]string{entity, sampler, stream, "signOn"}, ".")
	args := []valueArray{valueArray{Int: seconds}}

	return geneos.methodWithArgs(method, args)
}

func (geneos Client) signOffStream(entity string, sampler string, stream string) (err error) {
	method := strings.Join([]string{entity, sampler, stream, "signOn"}, ".")

	return geneos.methodNoArgs(method)
}

func (geneos Client) heartbeatStream(entity string, sampler string, stream string) (err error) {
	method := strings.Join([]string{entity, sampler, stream, "heartBeat"}, ".")

	return geneos.methodNoArgs(method)
}

func (geneos Client) gatewayConnected() (bool, error) {
	method := "_netprobe.gatewayConnected"

	return geneos.methodBoolNoArgs(method)
}

func (geneos Client) entityExists(entity string) (bool, error) {
	method := "_netprobe.managedEntityExists"
	args := []valueArray{valueArray{String: entity}}

	return geneos.methodBoolWithArgs(method, args)
}

func (geneos Client) samplerExists(entity string, sampler string) (result bool, err error) {
	method := "_netprobe.samplerExists"
	args := []valueArray{valueArray{String: entity + "." + sampler}}

	return geneos.methodBoolWithArgs(method, args)
}

/*

Gateway:

Old GW 1 function - not implemented

_gateway.addManagedEntity(string managedEntity, string dataSection)

*/
