package qogs

import (
	"text/template"
)

// TemplateFuncs returns a function map for use with text/template.
//
// The following functions are available:
//
//  contains(haystack, needle)   Check if needle occurs in the haystack
//  keys(map)                    Get all the keys in the given map
//  sortOn(container, path)      Sort the container values by the given path
//  reverse(container)           Reverse the order of the container values
func TemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"contains": func(haystack interface{}, needle interface{}) bool {
			return Contains(haystack, needle)
		},
		"keys": func(data interface{}) []interface{} {
			return Keys(data)
		},
		"sortOn": func(data interface{}, path string) []interface{} {
			return SortOn(Values(data), path)
		},
		"reverse": func(data interface{}) []interface{} {
			return Reverse(data)
		},
	}
}