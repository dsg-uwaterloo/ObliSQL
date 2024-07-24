package main

import (
	"fmt"
	"log"
	"strconv"
	"time"
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
func findStringIntersection(keyMap map[string][]string) []string {
	if len(keyMap) == 0 {
		return []string{}
	}

	// Get the first slice to start the intersection
	var result []string
	for _, slice := range keyMap {
		result = slice
		break
	}

	// Function to find the intersection of two slices
	intersection := func(a, b []string) []string {
		m := make(map[string]bool)
		for _, v := range a {
			m[v] = true
		}
		var intersect []string
		for _, v := range b {
			if m[v] {
				intersect = append(intersect, v)
			}
		}
		return intersect
	}

	// Iterate over the map and find the intersection
	for _, slice := range keyMap {
		result = intersection(result, slice)
	}

	return result
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

func getIndexFromArray[T comparable](arr []T, val T) int {
	for i, v := range arr {
		if v == val {
			return i
		}
	}
	return -1
}

func getDatesInRange(startDateStr, endDateStr string) ([]string, error) {
	// Parse the startDate and endDate strings into time.Time
	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid start date: %v", err)
	}
	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid end date: %v", err)
	}

	// Check if startDate is after endDate
	if startDate.After(endDate) {
		return nil, fmt.Errorf("start date must be before end date")
	}

	var dates []string
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d.Format("2006-01-02"))
	}
	return dates, nil
}

func detectRepeats(arr []string) []string {
	elementCount := make(map[string]int)
	var repeats []string

	for _, value := range arr {
		elementCount[value]++
		if elementCount[value] == 2 {
			repeats = append(repeats, value)
		}
	}

	return repeats
}
