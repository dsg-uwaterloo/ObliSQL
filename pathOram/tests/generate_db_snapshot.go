package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"pathOram/pkg/oram/oram"
	"pathOram/pkg/oram/request"
	"pathOram/pkg/oram/utils"

	"github.com/schollz/progressbar/v3"
)

const (
	// Z is the number of blocks per bucket.
	Z = 4
	// stashSize is the maximum number of blocks in the stash.
	stashSize = 7000000
)

func main() {
	// Define command-line flags.
	logCapPtr := flag.Int("logCapacity", 20, "Log capacity for ORAM")
	traceFilePath := flag.String("tracefile", "", "Path to the trace file")
	batchSizePtr := flag.Int("batchSize", 500, "Batch size for setting values in DB")
	flag.Parse()

	// Validate required flags.
	if *traceFilePath == "" {
		log.Fatal("Trace file path must be provided using the -tracefile flag")
	}

	logCapacity := *logCapPtr
	batchSize := *batchSizePtr

	// Create a file to store the CPU profile.
	// f, err := os.Create("cpu.prof")
	// if err != nil {
	// 	log.Fatalf("could not create CPU profile: %v", err)
	// }
	// // Start CPU profiling.
	// if err := pprof.StartCPUProfile(f); err != nil {
	// 	log.Fatalf("could not start CPU profile: %v", err)
	// }
	// // Ensure the profile is stopped on program exit.
	// defer pprof.StopCPUProfile()

	// // Set up a channel to listen for termination signals.
	// sigs := make(chan os.Signal, 1)
	// signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	// go func() {
	// 	sig := <-sigs
	// 	log.Printf("Received signal: %v. Stopping CPU profile and exiting...", sig)
	// 	pprof.StopCPUProfile()
	// 	f.Close()
	// 	os.Exit(0)
	// }()

	key_input := "oblisqloram"
	// Generate the SHA-256 hash of the input string.
	hash := sha256.New()
	hash.Write([]byte(key_input))
	// Return the 256-bit (32-byte) hash as a byte slice.
	key := hash.Sum(nil)

	fmt.Println("Building ORAM snapshot with logCapacity:", logCapacity)
	fmt.Println("Building ORAM snapshot with Z value:", Z)

	// Initialize ORAM.
	o, err := oram.NewORAM(logCapacity, Z, stashSize, "127.0.0.1:6379", false, key)
	if err != nil {
		log.Fatalf("Error initializing ORAM: %v", err)
	}
	defer o.RedisClient.Close()

	fmt.Println("Loading data from tracefile now:")

	// Load data from tracefile and create Request objects.
	var requests []request.Request
	file, err := os.Open(*traceFilePath)
	if err != nil {
		log.Fatalf("failed to open tracefile: %v", err)
	}
	defer file.Close()

	const maxBufferSize = 1024 * 1024 // 1MB
	scanner := bufio.NewScanner(file)
	buffer := make([]byte, maxBufferSize)
	scanner.Buffer(buffer, maxBufferSize)

	for scanner.Scan() {
		line := scanner.Text()
		// Only process lines that start with "SET".
		if strings.HasPrefix(line, "SET") {
			parts := strings.SplitN(line, " ", 3)
			if len(parts) != 3 {
				continue // skip lines that don't have exactly 3 parts.
			}
			key := parts[1]
			value := parts[2]
			requests = append(requests, request.Request{Key: key, Value: value})
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("error reading tracefile: %v", err)
	}

	fmt.Println("Finished scanning tracefile")
	fmt.Println("Setting values in DB...")

	// Initialize DB with tracefile contents and display a progress bar.
	bar := progressbar.Default(int64(len(requests)), "Setting values...")

	for start := 0; start < len(requests); start += batchSize {
		end := start + batchSize
		if end > len(requests) {
			end = len(requests) // Ensure we don't go out of bounds.
		}

		o.Batching(requests[start:end], batchSize)
		// Increment the progress bar by the batch size or remaining items.
		_ = bar.Add(end - start)
	}

	bar.Finish()
	fmt.Println("Finished Initializing DB!")

	// Print the stash map to inspect the internal state of ORAM.
	fmt.Println("Printing the stash...")
	utils.PrintStashMap(o)

	// =============================================================
	// Storing Keymap and StashMap in a JSON file.
	data := map[string]interface{}{
		"Keymap":   o.Keymap,
		"StashMap": o.StashMap,
	}

	// Open or create the snapshot file.
	file, err = os.Create("proxy_snapshot.json")
	if err != nil {
		log.Fatalf("Error creating file: %v", err)
	}
	defer file.Close()

	// Serialize data to JSON and write to the file.
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // Indentation for better readability.
	if err := encoder.Encode(data); err != nil {
		log.Fatalf("Error encoding data to JSON: %v", err)
	}

	fmt.Println("Data written to file: proxy_snapshot.json")

	// =============================================================

	// Trigger a save to dump.rdb.
	fmt.Println("Triggering RDB snapshot save (SAVE)...")
	err = o.RedisClient.Client.Save(o.RedisClient.Ctx).Err() // SAVE command.
	if err != nil {
		log.Fatalf("Error triggering RDB snapshot save: %v", err)
	}
	fmt.Println("RDB snapshot save triggered successfully.")
}
