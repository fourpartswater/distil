package compute

import (
	"encoding/csv"
	"io"
	"os"

	"github.com/pkg/errors"
)

// ParseResultCSV parses a result CSV that is compliant with RFC 4180, with
// additional logic added to extract nested arrays generated by PANDAS to_csv() calls.
func ParseResultCSV(path string) ([][]interface{}, error) {
	csvFile, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "error opening result file")
	}

	csvReader := csv.NewReader(csvFile)
	results := [][]interface{}{}
	for {
		line, err := csvReader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, errors.Wrap(err, "error parsing result file")
		}

		record := []interface{}{}
		for _, elem := range line {
			// parse value into float, int, string, or array
			record = append(record, parseVal(elem))
		}
		results = append(results, record)
	}
	return results, nil
}

func parseVal(val string) interface{} {
	// check to see if we can parse the value as an array - if not we leave it as a string
	arrayVal, err := parseArray(val)
	if err == nil {
		return arrayVal
	}
	return val
}

func parseArray(val string) ([]interface{}, error) {
	field := &ComplexField{
		Buffer: val,
	}
	field.Init()

	err := field.Parse()
	if err != nil {
		return nil, err
	}

	field.Execute()
	return field.arrayElements.elements, nil
}

// Structure to interact with peg parser
type arrayElements struct {
	elements []interface{}
	stack    [][]interface{}
}

func (a *arrayElements) lastIdx() int {
	return len(a.stack) - 1
}

// Called by peg parse
func (a *arrayElements) addElement(element interface{}) {
	a.stack[a.lastIdx()] = append(a.stack[a.lastIdx()], element)
}

func (a *arrayElements) pushArray() {
	a.stack = append(a.stack, []interface{}{})
}

func (a *arrayElements) popArray() {
	a.elements, a.stack = a.stack[a.lastIdx()], a.stack[:a.lastIdx()]
	if len(a.stack) != 0 {
		a.stack[a.lastIdx()] = append(a.stack[a.lastIdx()], a.elements)
	}
}
