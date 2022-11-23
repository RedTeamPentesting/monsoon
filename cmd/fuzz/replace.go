package fuzz

import (
	"fmt"
	"strings"
)

type Replace struct {
	Name    string
	Type    string
	Options string
}

// ParseReplace parses a replace rule.
func ParseReplace(s string) (r Replace, err error) {
	data := strings.SplitN(s, ":", 3)

	if len(data) != 3 {
		return Replace{}, fmt.Errorf("invalid format for replace, want NAME:type:options")
	}

	r = Replace{
		Name:    data[0],
		Type:    data[1],
		Options: data[2],
	}

	return r, nil
}
