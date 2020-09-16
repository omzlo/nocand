package properties

import (
	"encoding/json"
	"fmt"
)

//
// Properties
//

const (
	PROP_TYPE_NULL byte = iota
	PROP_TYPE_BOOL
	PROP_TYPE_STRING
	PROP_TYPE_INT32
	PROP_TYPE_UINT32
	PROP_TYPE_FLOAT32
)

type Property struct {
	Type  byte        `json:"-"`
	Value interface{} `json:"value"`
}

type Properties struct {
	Map map[string]Property
}

func New() *Properties {
	return &Properties{Map: make(map[string]Property)}
}

func (si *Properties) Clear() {
	// optimized by compiler since Go 1.11
	for k := range si.Map {
		delete(si.Map, k)
	}
}

func (si *Properties) Add(k string, t byte, v interface{}) bool {
	if len(k) > 127 {
		return false
	}
	si.Map[k] = Property{t, v}
	return true
}

func (si *Properties) AddNil(k string) bool {
	return si.Add(k, PROP_TYPE_NULL, nil)
}

func (si *Properties) AddString(k string, v string) bool {
	return si.Add(k, PROP_TYPE_STRING, v)
}

func (si *Properties) AddInt32(k string, v int32) bool {
	return si.Add(k, PROP_TYPE_INT32, v)
}

func (si *Properties) AddUint32(k string, v uint32) bool {
	return si.Add(k, PROP_TYPE_UINT32, v)
}

func (si *Properties) AddFloat32(k string, v float32) bool {
	return si.Add(k, PROP_TYPE_FLOAT32, v)
}

func (si *Properties) AddBool(k string, v bool) bool {
	return si.Add(k, PROP_TYPE_BOOL, v)
}

func (si *Properties) MarshalJSON() ([]byte, error) {
	return json.Marshal(si.Map)
}

func (si *Properties) AsString(k string) string {
	if x, ok := si.Map[k]; ok {
		return fmt.Sprintf("%v", x.Value)
	}
	return ""
}
