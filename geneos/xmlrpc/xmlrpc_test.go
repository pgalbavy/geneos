package xmlrpc // import "wonderland.org/geneos/xmlrpc"

import (
	"fmt"
	"testing"
	"time"
)

var geneos Client
var sampler Sampler

func TestConnect(t *testing.T) {
	var err error
	sampler, err = ConnectSampler("ubuntu", "xml-rpc", "ubuntu", 7036)
	if err != nil {
		t.Errorf("new failed: %v", err)
	}
}

func TestEntityExists(t *testing.T) {
	res, err := geneos.EntityExists("ubuntu")
	if err != nil {
		t.Errorf("Error checking for entity \"ubuntu\": %v", err)
	}
	if res != true {
		t.Errorf("Entity \"ubuntu\" not found")
	}
	res, err = geneos.EntityExists("rubbish")
	if err != nil {
		t.Errorf("Error checking for entity \"rubbish\": %v", err)

	}
	if res != false {
		t.Errorf("Entity \"rubbish\" found when it should not exist")
	}
}

func TestSamplerExists(t *testing.T) {
	res, err := geneos.SamplerExists("ubuntu", "xml-rpc")
	if err != nil {
		t.Errorf("Error checking for sampler \"xml-rpc\": %v", err)
	}
	if res != true {
		t.Errorf("Sampler \"xml-rpc\" not found")
	}
	res, err = geneos.SamplerExists("ubuntu", "rubbish")
	if err != nil {
		t.Errorf("Error checking for sampler \"rubbish\": %v", err)

	}
	if res != false {
		t.Errorf("Sampler \"rubbish\" found when it should not exist")
	}
}

func TestViewExists(t *testing.T) {
	res, err := geneos.viewExists("ubuntu", "xml-rpc", "TEST", "")
	if err != nil {
		t.Errorf("Error checking for view \"TEST\": %v", err)
	}
	if res != true {
		t.Errorf("View \"TEST\" not found")
	}
}

func TestRemoveView(t *testing.T) {
	err := geneos.removeView("ubuntu", "xml-rpc", "TEST", "")
	if err != nil {
		t.Errorf("Error removing view \"TEST\": %v", err)
	}
}

func TestCreateView(t *testing.T) {
	err := geneos.CreateView("ubuntu", "xml-rpc", "TEST", "")
	if err != nil {
		t.Errorf("Error creating dataview: %v\n", err)
	}
}

func TestGetParameter(t *testing.T) {
	res, err := geneos.GetParameter("ubuntu", "xml-rpc", "TEST")
	if err != nil {
		t.Errorf("Error getting parameter: %v\n", err)
	}
	if res != "1234" {
		t.Errorf("Parameter not valid: %q\n", res)
	}
}

/* func TestRemoveTableRow(t *testing.T) {
	err := geneos.removeTableRow("ubuntu", "xml-rpc", "TEST", "", "newrow")
	if err != nil {
		t.Errorf("Error removing row: %v\n", err)
	}
} */

func TestAddTableRow(t *testing.T) {
	err := geneos.AddTableRow("ubuntu", "xml-rpc", "TEST", "", "newrow")
	if err != nil {
		t.Errorf("Error adding row: %v\n", err)
	}
}

func TestAddTableColumn(t *testing.T) {
	err := geneos.AddTableColumn("ubuntu", "xml-rpc", "TEST", "", "column1")
	if err != nil {
		t.Errorf("Error adding column1: %v\n", err)
	}
	err = geneos.AddTableColumn("ubuntu", "xml-rpc", "TEST", "", "column2")
	if err != nil {
		t.Errorf("Error adding column2: %v\n", err)
	}
}

func TestUpdateVariable(t *testing.T) {
	err := geneos.UpdateVariable("ubuntu", "xml-rpc", "TEST", "", "newrow.column2", "value")
	if err != nil {
		t.Errorf("Error adding row: %v\n", err)
	}
}

func TestUpdateTableRow(t *testing.T) {
	row := []string{"value1"}
	err := geneos.UpdateTableRow("ubuntu", "xml-rpc", "TEST", "", "newrow", row)
	if err != nil {
		t.Errorf("Error updating row: %v\n", err)
	}
}

func TestColumnExists(t *testing.T) {
	res, err := geneos.ColumnExists("ubuntu", "xml-rpc", "TEST", "", "columnX")
	if err != nil {
		t.Errorf("Error checking column: %v\n", err)
	}
	if res != false {
		t.Errorf("Column \"columnX\" should not exist")
	}

	res, err = geneos.ColumnExists("ubuntu", "xml-rpc", "TEST", "", "column1")
	if err != nil {
		t.Errorf("Error checking column: %v\n", err)
	}
	if res != true {
		t.Errorf("Column \"columnX\" should exist")
	}
}

func TestGetColumnCount(t *testing.T) {
	res, err := geneos.GetColumnCount("ubuntu", "xml-rpc", "TEST", "")
	if err != nil {
		t.Errorf("Error getting column count: %v\n", err)
	}
	if res != 2 {
		t.Errorf("Only 2 columns should not exist")
	}
}

func TestGetColumnNames(t *testing.T) {
	res, err := geneos.GetColumnNames("ubuntu", "xml-rpc", "TEST", "")
	if err != nil {
		t.Errorf("Error getting column names: %v\n", err)
	}
	fmt.Printf("columns=%+v\n", res)

}

func TestGetRowNames(t *testing.T) {
	res, err := geneos.GetRowNames("ubuntu", "xml-rpc", "TEST", "")
	if err != nil {
		t.Errorf("Error getting row names: %v\n", err)
	}
	fmt.Printf("rows=%+v\n", res)

}

func TestGetHeadlineNames(t *testing.T) {
	res, err := geneos.GetHeadlineNames("ubuntu", "xml-rpc", "TEST", "")
	if err != nil {
		t.Errorf("Error getting headline names: %v\n", err)
	}
	fmt.Printf("headlines=%+v\n", res)

}

func TestGetRowNamesOlder(t *testing.T) {
	res, err := geneos.GetRowNamesOlderThan("ubuntu", "xml-rpc", "TEST", "", int(time.Now().Unix()))
	if err != nil {
		t.Errorf("Error getting row names: %v\n", err)
	}
	fmt.Printf("rows=%+v\n", res)

}

func TestUpdateEntireTable(t *testing.T) {
	table := [][]string{
		{"column1", "column2"},
		{"value3", "value4"},
		{"value5", "value4"},
		{"value6", "value4"},
		{"value7", "value4"},
	}
	err := geneos.UpdateEntireTable("ubuntu", "xml-rpc", "TEST", "", table)
	if err != nil {
		t.Errorf("Error updating row: %v\n", err)
	}
}

func TestDataview(t *testing.T) {
	table := [][]string{
		{"column1", "column2", "column3"},
		{"row1"},
		{"row2"},
		{"row3"},
		{"row4"},
	}
	_, err := sampler.Dataview("TEST8", table)
	if err != nil {
		t.Errorf("Error creating dataview: %v\n", err)
	}
}
