package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"hash/fnv"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
)

type KVpair struct {
	Key   string
	Value string
}

type TableConfig struct {
	Name        string `json:"name"`
	PartitionID uint32 `json:"partition_id"`
}

type Config struct {
	Tables             []TableConfig `json:"tables"`
	TotalPartitions    uint32        `json:"total_partitions"`
	DefaultPartitioning string       `json:"default_partitioning"`
}

func hashString(s string, N uint32) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32() % N
}

func loadConfig(configPath string) (*Config, error) {
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func assignTableToFile(tableName string, config *Config) uint32 {
	// First check if table is explicitly configured
	for _, table := range config.Tables {
		if table.Name == tableName {
			return table.PartitionID % config.TotalPartitions
		}
	}

	// For unknown tables, use the default partitioning strategy
	if config.DefaultPartitioning == "hash" {
		h := fnv.New32a()
		h.Write([]byte(tableName))
		return h.Sum32() % config.TotalPartitions
	}

	// Default fallback to partition 0
	return 0
}

// Extract table name from key (first part before '/')
func extractTableName(key string) string {
	parts := strings.Split(key, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return key // fallback to full key if no '/' found
}

func readTrace(filePath string) []KVpair {
	data := []KVpair{}

	file, err := os.Open(filePath)
	if err != nil {
		log.Fatal().Msgf("Error opening metadata file: %s", err)
	}
	defer file.Close()

	// Create a scanner with a larger buffer
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 1024*1024) // 1 MB buffer
	scanner.Buffer(buf, 10*1024*1024) // Set max buffer size to 10 MB

	totalKeys := 0
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, " ")
		if len(parts) == 3 {
			op := parts[0]
			if op == "SET" {
				totalKeys++
				key := parts[1]
				value := parts[2]
				newPair := KVpair{Key: key, Value: value}
				data = append(data, newPair)
			}
		} else {
			log.Info().Msg("Invalid! " + strings.Join(parts, " "))
		}
	}
	return data
}

func writeToFile(filename string, kv KVpair) {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal().Msgf("Error opening file for writing: %s", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	_, err = writer.WriteString("SET" + " " + kv.Key + " " + kv.Value + "\n")
	if err != nil {
		log.Fatal().Msgf("Error writing to file: %s", err)
	}
	writer.Flush()
}

func main() {
	traceLoc := flag.String("t", "./serverInput.txt", "Trace file location")
	configLoc := flag.String("c", "./table_config.json", "Table configuration file")
	flag.Parse()

	// Load configuration
	config, err := loadConfig(*configLoc)
	if err != nil {
		log.Fatal().Msgf("Error loading config: %s", err)
	}

	data := readTrace(*traceLoc)
	fmt.Println("Read: ", len(data))
	N := config.TotalPartitions

	// Create Output directory if it doesn't exist
	outputDir := "Output"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatal().Msgf("Error creating output directory: %s", err)
	}

	// Create file writers for each bucket
	fileWriters := make(map[uint32]string)
	for i := uint32(0); i < N; i++ {
		fileWriters[i] = outputDir + "/serverInput_" + string(rune('0'+i)) + ".txt"
	}

	// Track which tables go to which files for debugging
	tableToFile := make(map[string]uint32)

	// Distribute data into files based on table name
	for _, kv := range data {
		tableName := extractTableName(kv.Key)
		hashValue := assignTableToFile(tableName, config)
		filename := fileWriters[hashValue]

		// Track table distribution for debugging
		if _, exists := tableToFile[tableName]; !exists {
			tableToFile[tableName] = hashValue
			fmt.Printf("Table '%s' -> File %d (%s)\n", tableName, hashValue, filename)
		}

		writeToFile(filename, kv)
	}

	// Print summary
	fmt.Printf("\nDistribution summary:\n")
	for table, fileIdx := range tableToFile {
		fmt.Printf("Table '%s' -> Output/serverInput_%d.txt\n", table, fileIdx)
	}
}
