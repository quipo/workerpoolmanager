package utils

import (
	"fmt"
	"io/ioutil"
)

// ListFiles lists all the files in a given directory
func ListFiles(dir string) []string {
	filenames := make([]string, 0)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		fmt.Println(err)
	} else {
		for _, file := range files {
			filenames = append(filenames, file.Name())
		}
	}
	return filenames
}

// Filter elements in a string array with a certain function
func Filter(vs []string, f func(string) bool) []string {
	vsf := make([]string, 0)
	for _, v := range vs {
		if f(v) {
			vsf = append(vsf, v)
		}
	}
	return vsf
}

// Map calls a function on all the items in a string array
func Map(vs []string, f func(string) string) []string {
	vsm := make([]string, len(vs))
	for i, v := range vs {
		vsm[i] = f(v)
	}
	return vsm
}

// KMap calls a function on all the items in a string array and returns
// the results as a map, containing the original string as key
func KMap(vs []string, f func(string) string) map[string]string {
	vsm := make(map[string]string)
	for _, v := range vs {
		vsm[v] = f(v)
	}
	return vsm
}
