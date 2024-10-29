package resolver

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	loadbalancer "github.com/project/ObliSql/api/loadbalancer"
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

// Takes two lists
func FindCommonElements[T comparable](list1, list2 []T) []T {
	elementMap := make(map[T]bool)
	commonElements := []T{}

	// Add elements from the first list to the map
	for _, item := range list1 {
		elementMap[item] = true
	}

	// Check if elements from the second list exist in the map
	for _, item := range list2 {
		if elementMap[item] {
			commonElements = append(commonElements, item)
		}
	}

	return commonElements
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

func createMapFromLists(keys, values []string) map[string]map[string]string {
	result := make(map[string]map[string]string)

	for i := 0; i < len(keys); i++ {
		parts := strings.Split(keys[i], "/")

		colName := parts[1]
		pkVal := parts[2]
		colVal := values[i]

		if _, exists := result[pkVal]; !exists {
			result[pkVal] = make(map[string]string)
		}
		result[pkVal][colName] = colVal
	}

	return result
}

func sortMapByColumn(data map[string]map[string]string, sortColumn, columnType, order string) []map[string]string {

	supportedColumnTypes := []string{"int", "date", "varchar"}

	if !contains(supportedColumnTypes, columnType) {
		log.Fatalf("Sorting on Column Type: %s is not supported! Please check supported types in function sortMapByColumn in Utils\n", columnType)
	}

	sortedData := make([]map[string]string, 0, len(data))

	// Convert the map to a slice of maps
	for id, values := range data {
		entry := map[string]string{"id": id}
		for k, v := range values {
			entry[k] = v
		}
		sortedData = append(sortedData, entry)
	}

	// Sort the slice based on the specified column, type, and order
	sort.Slice(sortedData, func(i, j int) bool {
		valueI := sortedData[i][sortColumn]
		valueJ := sortedData[j][sortColumn]

		switch columnType {
		case "int":
			intI, errI := strconv.Atoi(valueI)
			intJ, errJ := strconv.Atoi(valueJ)
			if errI == nil && errJ == nil {
				if order == "ASC" {
					return intI < intJ
				} else if order == "DESC" {
					return intI > intJ
				}
			}
		case "date":
			dateI, errI := time.Parse("2006-01-02", valueI)
			dateJ, errJ := time.Parse("2006-01-02", valueJ)
			if errI == nil && errJ == nil {
				if order == "ASC" {
					return dateI.Before(dateJ)
				} else if order == "DESC" {
					return dateI.After(dateJ)
				}
			}
		case "varchar":
			if order == "ASC" {
				return valueI < valueJ
			} else if order == "DESC" {
				return valueI > valueJ
			}
		}
		return false // Default to not swapping if order or type is invalid
	})

	return sortedData
}

func prettyPrintMap(data []map[string]string) {
	fmt.Println("{")
	for _, entry := range data {
		id := entry["id"]
		fmt.Printf("  %s: {\n", id)
		keys := make([]string, 0, len(entry))
		for k := range entry {
			if k != "id" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for i, key := range keys {
			if i == len(keys)-1 {
				fmt.Printf("    %s: %s\n", key, entry[key])
			} else {
				fmt.Printf("    %s: %s,\n", key, entry[key])
			}
		}
		fmt.Println("  }")
	}
	fmt.Println("}")
}

func (c *myResolver) simpleFetch(keys []string, val []string, reqId int64) ([]string, []string) {

	ctx := context.Background()
	indexReqKeys := loadbalancer.LoadBalanceRequest{
		Keys:      keys,
		Values:    val,
		RequestId: reqId,
	}
	resp, err := c.conn.AddKeys(ctx, &indexReqKeys)
	if err != nil {
		log.Fatalf("Failed to fetch from load balancer! %s \n", err)
	}
	return resp.Keys, resp.Values
}

func (c *myResolver) getListFromInterface(pkList interface{}) []string {
	interfaceList, ok := pkList.([]interface{})
	if !ok {
		fmt.Printf("Unexpected type for pkList: %T\n", pkList)
		return nil
	}
	newList := make([]string, len(interfaceList))
	for i, v := range interfaceList {
		switch value := v.(type) {
		case string:
			newList[i] = value
		case float64:
			newList[i] = fmt.Sprintf("%.0f", value) // Convert float64 to string without decimal places
		default:
			fmt.Printf("Unexpected type for element: %T\n", v)
			newList[i] = fmt.Sprintf("%v", v) // Convert to string representation as fallback
		}
	}
	return newList
}
