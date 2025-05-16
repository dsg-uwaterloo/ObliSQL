package main

import (
	"encoding/csv"
	"os"
	"strconv"
	"time"

	"math/rand"

	"github.com/pingcap/go-ycsb/pkg/generator"
)

func generateSkewedIdList(zipfFactor float64) {
	zipfScrambed := generator.NewZipfianWithRange(10000, 30000, zipfFactor)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	file, err := os.Create("skewed_ids_" + strconv.FormatFloat(zipfFactor, 'f', -1, 64) + ".csv")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	for i := 0; i < 1000000; i++ {
		id := zipfScrambed.Next(r)
		err := writer.Write([]string{strconv.Itoa(int(id))})
		if err != nil {
			panic(err)
		}
	}
}

func main() {
	// generateSkewedIdList(0.99) //High Skew
	// generateSkewedIdList(0.55) //Medium Skew
	// generateSkewedIdList(0.11) //Low Skew
	generateSkewedIdList(0.00)
}
