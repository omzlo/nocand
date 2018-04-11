package models

import (
	"encoding/xml"
	"strings"
)

var NodeInfo *NodeDirectory = NewNodeDirectory()

type NodeAttributes map[string]string

type NodeDirectory struct {
	Entries map[Udid8]NodeAttributes
}

func NewNodeDirectory() *NodeDirectory {
	return &NodeDirectory{Entries: make(map[Udid8]NodeAttributes)}
}

func (dir NodeDirectory) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	/* ... */
	return nil
}

func (dir *NodeDirectory) SetAttribute(udid Udid8, key string, value string) bool {
	_, ok := dir.Entries[udid]
	if !ok {
		dir.Entries[udid] = make(map[string]string)
	}
	dir.Entries[udid][key] = value
	return true
}

func (dir *NodeDirectory) GetAttribute(udid Udid8, key string) (string, bool) {
	attributes, ok := dir.Entries[udid]
	if ok {
		val, ok := attributes[key]
		return val, ok
	}
	return "", false
}

func (dir *NodeDirectory) ExpandAttributes(udid Udid8, s string) string {
	var result string

	attributes, ok := dir.Entries[udid]
	if ok {
		for {
			idx := strings.Index(s, "$(")
			if idx < 0 {
				return result + s
			}
			result += s[:idx]
			s = s[idx:]
			idx = strings.Index(s, ")")
			if idx < 0 {
				return result + s
			}
			key := s[2:idx]
			result += attributes[key]
			s = s[idx+1:]
		}
	}
	return s
}
