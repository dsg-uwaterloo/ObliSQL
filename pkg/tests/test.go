package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
)

type MetaData struct {
	ColNames  []string `json:"colNames"`
	IndexOn   []string `json:"indexOn"`
	PkEnd     int      `json:"pkEnd"`
	PkStart   int      `json:"pkStart"`
	TableName string   `json:"tableName"`
}

func main() {
	data := make(map[string]MetaData)

	file, err := os.Open("./metadata.txt")
	if err != nil {
		log.Fatalf("Error opening metadata file: %s\n", err)
		return
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("Error reading metadata file: %s\n", err)
		return
	}

	err = json.Unmarshal(byteValue, &data)
	if err != nil {
		log.Fatalf("Error unmarshaling JSON: %s\n", err)
		return
	}

	fmt.Println(data)
}
