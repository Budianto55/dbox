package csv

import (
	"encoding/csv"
	// "encoding/json"
	"errors"
	"fmt"
	"github.com/eaciit/cast"
	"github.com/eaciit/dbox"
	"github.com/eaciit/errorlib"
	"github.com/eaciit/toolkit"
	"io"
	"os"
	"reflect"
	"strings"
)

const (
	modCursor = "Cursor"
)

// type ConditionAttr struct {
// 	Find   toolkit.M
// 	Select toolkit.M
// 	Sort   []string
// 	skip   int
// 	limit  int
// }

type Cursor struct {
	dbox.Cursor

	ResultType   string
	count        int
	file         *os.File
	reader       *csv.Reader
	ConditionVal QueryCondition

	headerColumn []headerstruct
}

func (c *Cursor) Close() {
}

func (c *Cursor) validate() error {
	if c.reader == nil {
		return errors.New(fmt.Sprintf("Reader is nil"))
	}

	return nil
}

func (c *Cursor) prepIter() error {
	e := c.validate()
	if e != nil {
		return e
	}
	return nil
}

func (c *Cursor) Count() int {
	return c.count
}

func (c *Cursor) ResetFetch() error {
	var e error
	c.Connection().(*Connection).Close()
	e = c.Connection().(*Connection).Connect()

	if e != nil {
		return errorlib.Error(packageName, modCursor, "Restart Connection", e.Error())
	}

	c.headerColumn = c.Connection().(*Connection).headerColumn
	c.file = c.Connection().(*Connection).file
	c.reader = c.Connection().(*Connection).reader

	e = c.prepIter()
	if e != nil {
		return errorlib.Error(packageName, modCursor, "ResetFetch", e.Error())
	}

	// c.PrepareCursor()
	// if e != nil {
	// 	return errorlib.Error(packageName, modCursor, "Prepare Cursor", e.Error())
	// }

	return nil
}

// func (c *Cursor) PrepareCursor() error {
// 	var e error

// 	c.headerColumn, e = c.reader.Read()
// 	if e != nil {
// 		return e
// 	}
// 	return nil
// }

func (c *Cursor) Fetch(m interface{}, n int, closeWhenDone bool) error {

	if closeWhenDone {
		defer c.Close()
	}

	e := c.prepIter()
	if e != nil {
		return errorlib.Error(packageName, modCursor, "Fetch", e.Error())
	}

	if !toolkit.IsPointer(m) {
		return errorlib.Error(packageName, modCursor, "Fetch", "Model object should be pointer")
	}

	if n != 1 && reflect.ValueOf(m).Elem().Kind() != reflect.Slice {
		return errorlib.Error(packageName, modCursor, "Fetch", "Model object should be pointer of slice")
	}

	var v reflect.Type

	if n == 1 {
		v = reflect.TypeOf(m).Elem()
	} else {
		v = reflect.TypeOf(m).Elem().Elem()
	}

	ivs := reflect.MakeSlice(reflect.SliceOf(v), 0, 0)

	lineCount := 0

	//=============================
	// fmt.Println("Qursor 133 : ", c.ConditionVal.Find)
	for {
		iv := reflect.New(v).Interface()

		isAppend := true
		c.count += 1
		recData := toolkit.M{}
		appendData := toolkit.M{}

		dataTemp, e := c.reader.Read()

		for i, val := range dataTemp {
			orgname := c.headerColumn[i].name
			lowername := strings.ToLower(c.headerColumn[i].name)

			switch c.headerColumn[i].dataType {
			case "int":
				recData[lowername] = cast.ToInt(val, cast.RoundingAuto)
			case "float":
				recData[lowername] = cast.ToF64(val, 2, cast.RoundingAuto)
			default:
				recData[lowername] = val
			}

			if len(c.ConditionVal.Select) == 0 || c.ConditionVal.Select.Get("*", 0).(int) == 1 {
				appendData[orgname] = recData[lowername]
			} else {
				if c.ConditionVal.Select.Get(strings.ToLower(c.headerColumn[i].name), 0).(int) == 1 {
					appendData[orgname] = recData[lowername]
				}
			}
		}

		isAppend = c.ConditionVal.getCondition(recData)

		if c.count < c.ConditionVal.skip || (c.count > (c.ConditionVal.skip+c.ConditionVal.limit) && c.ConditionVal.limit > 0) {
			isAppend = false
		}

		if v.Kind() == reflect.Struct {
			for i := 0; i < v.NumField(); i++ {
				if appendData.Has(v.Field(i).Name) {
					switch v.Field(i).Type.Kind() {
					case reflect.Int:
						appendData.Set(v.Field(i).Name, cast.ToInt(appendData[v.Field(i).Name], cast.RoundingAuto))
					}
				}
			}
		}

		if e == io.EOF {
			if isAppend && len(appendData) > 0 {
				toolkit.Serde(appendData, iv, "json")
				ivs = reflect.Append(ivs, reflect.ValueOf(iv).Elem())
				lineCount += 1
			}
			break
		} else if e != nil {
			return errorlib.Error(packageName, modCursor,
				"Fetch", e.Error())
		}

		if isAppend && len(appendData) > 0 {
			toolkit.Serde(appendData, iv, "json")
			ivs = reflect.Append(ivs, reflect.ValueOf(iv).Elem())
			lineCount += 1
		}

		if n > 0 {
			if lineCount >= n {
				break
			}
		}
	}

	if e != nil {
		return errorlib.Error(packageName, modCursor, "Fetch", e.Error())
	}

	if n == 1 {
		if ivs.Len() > 0 {
			reflect.ValueOf(m).Elem().Set(ivs.Index(0))
		}
	} else {
		reflect.ValueOf(m).Elem().Set(ivs)
	}

	return nil
}
