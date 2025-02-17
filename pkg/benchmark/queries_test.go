package benchmark

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"
)

func TestGetRandomRangeFromList(t *testing.T) {
	userIDFile := os.Getenv("USER_ID_FILE")
	if userIDFile == "" {
		userIDFile = "../../pkg/benchmark/benchmarkIdLists/u_id.csv"
	}
	user_id_list, err := ReadCSVColumn(userIDFile, 0, true)
	if err != nil {
		log.Fatal(err)
	}

	start, end := getRandomRangeFromList(&user_id_list, 5)

	fmt.Println("Start:", start, "End:", end)
}

func TestGetRandomRangeFromDate(t *testing.T) {
	startDate := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2060, 12, 31, 0, 0, 0, 0, time.UTC)

	start, end := getRandomDateRange(startDate, endDate, getRandomNumber(5))

	fmt.Println("Start:", start, "End:", end)
}
