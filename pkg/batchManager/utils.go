package batcher

import (
	"hash/fnv"
	"strings"
)

func hashString(s string, N uint32) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32() % N
}

func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

func separateKeysValues(pairs *[]string) ([]string, []string) {
	var keys []string
	var values []string

	for _, pair := range *pairs {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) == 2 {
			keys = append(keys, parts[0])
			values = append(values, parts[1])
		}
	}

	return keys, values
}
