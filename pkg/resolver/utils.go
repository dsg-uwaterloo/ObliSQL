package main

import (
	"log"
	"strconv"
)

func isInt(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func isFloat(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

func getBounds(s string, e string) (int64, int64) {
	//How do we select precision for floating point numbers?
	//Do we specify it for each db? (In the database)?
	if isInt(s) && isInt(e) {
		s_int, _ := strconv.ParseInt(s, 10, 64)
		e_int, _ := strconv.ParseInt(e, 10, 64)
		return s_int, e_int

	} else {
		log.Fatalf("Conversion of un-supported type")
		return 0, 0
	}
}
