package qogs

import (
	"fmt"
	"text/template"
)

// TemplateFuncs returns a function map for use with text/template.
//
// The following functions are available:
//
//  contains(haystack, needle)   Check if needle occurs in the haystack
//  keys(map)                    Get all the keys in the given map
//  sortOn(container, path)      Sort the container values by the given path
//  filterEq(container,          Get all container values where the value at
//    valuePath, comparePath)    valuePath equals comparePath of the container
//  reverse(container)           Reverse the order of the container values
//  limit(container, amount)     Keep only the first elements of the container
//
//  literal(str)                 Construct a path representing a string literal
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
		"filterEq": func(data interface{}, valuePath, comparePath interface{}) []interface{} {
			return FilterEq(data, fmt.Sprint(valuePath), fmt.Sprint(comparePath))
		},
		"reverse": func(data interface{}) []interface{} {
			return Reverse(data)
		},
		"limit": func(data interface{}, amount int) []interface{} {
			return Limit(data, amount)
		},

		"literal": func(str string) string {
			return "literal " + str
		},
	}
}
