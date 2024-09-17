package waffle_client

import (
	"fmt"
	"log"
	"reflect"
	"sync"
	"testing"
	"time"
)

// Padding all test cases to length of 8.
var testCases = []TestCase{
	{
		name:     "Valid Get Request",
		Keys:     []string{"K1", "K2", "K3", "K4", "K5", "K6", "K7", "K8"},
		Values:   []string{"", "", "", "", "", "", "", ""},
		expected: []string{"K1:V1", "K2:V2", "K3:V3", "K4:V4", "K5:V5", "K6:V6", "K7:V7", "K8:V8"},
	},
	{
		name:     "Get Non-existant Value",
		Keys:     []string{"K100", "K200", "K10000", "K10000", "K10000", "K10000", "K10000", "K10000"},
		Values:   []string{"", "", "", "", "", "", "", ""},
		expected: []string{"K100:V100", "K200:V200", "K10000:-1", "K10000:-1", "K10000:-1", "K10000:-1", "K10000:-1", "K10000:-1"},
	},
	{
		name:     "Normal Put Request",
		Keys:     []string{"K1", "K2", "K3", "K4", "K5", "K6", "K7", "K8"},
		Values:   []string{"VT1", "VT2", "VT3", "VT4", "VT5", "VT6", "VT7", "VT8"},
		expected: []string{"K1:VT1", "K2:VT2", "K3:VT3", "K4:VT4", "K5:VT5", "K6:VT6", "K7:VT7", "K8:VT8"},
	},
	{
		name:     "Get & Puts in One",
		Keys:     []string{"K1", "K2", "K1", "K3", "K1", "K1", "K1", "K1"},
		Values:   []string{"", "", "Test", "VT3", "", "Final", "", ""},
		expected: []string{"K1:VT1", "K2:VT2", "K1:Test", "K3:VT3", "K1:Test", "K1:Final", "K1:Final", "K1:Final"},
	},
	{
		name:     "Get & Puts With Missing Key",
		Keys:     []string{"K1", "K2", "K1", "k999999", "K3", "K1", "K1", "K1"},
		Values:   []string{"", "", "Test", "", "VT3", "", "Final", ""},
		expected: []string{"K1:Final", "K2:VT2", "K1:Test", "k999999:-1", "K3:VT3", "K1:Test", "K1:Final", "K1:Final"},
	},
	{
		name:     "Missing Key, then Inserted",
		Keys:     []string{"K999999", "K999999", "K999999", "K999999", "K999999", "K999999", "K999999", "K999999"},
		Values:   []string{"", "Hello", "", "", "", "", "", "", ""},
		expected: []string{"K999999:-1", "K999999:Hello", "K999999:Hello", "K999999:Hello", "K999999:Hello", "K999999:Hello", "K999999:Hello", "K999999:Hello"},
	},
}

type TestCase struct {
	name     string
	Keys     []string
	Values   []string
	expected []string
}

func generateDataSet() ([]string, []string) {
	Keys := []string{}
	Values := []string{}

	for i := 0; i < 1000; i++ {
		newKey := fmt.Sprintf("K%d", i)
		newValue := fmt.Sprintf("V%d", i)
		Keys = append(Keys, newKey)
		Values = append(Values, newValue)
	}

	return Keys, Values
}

func TestExecutor(t *testing.T) {
	// ctx := context.Background()
	proxyAddress := "localhost"
	proxyPort := 9090

	client := &ProxyClient{}
	err := client.Init(proxyAddress, proxyPort)

	if err != nil {
		log.Fatalf("Couldn't Connect to Executor Proxy. Error: %s", err)
	}

	client.InitArgs(10, 8, 2, 100, 1, 1)

	Keys, Values := generateDataSet()

	client.InitDB(Keys, Values)
	time.Sleep(1 * time.Second)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fmt.Printf("Running test: %s \n", tc.name)

			// Call the MixBatch function with the test case Keys and Values
			resp, err := client.MixBatch(tc.Keys, tc.Values)
			if err != nil {
				t.Errorf("MixBatch Error = %v", err)
				return
			}
			// fmt.Println(resp)
			// Compare the actual results with the expected results
			if !reflect.DeepEqual(resp, tc.expected) {
				t.Errorf("ExecuteBatch mismatch! \nExpected: %v, \nGot: %v \n", tc.expected, resp)
			}
		})
	}
}

type safeClient struct {
	client *ProxyClient
	mutex  sync.Mutex
}

// func TestExecutorMultiClient(t *testing.T) {
// 	proxyAddress := "localhost"
// 	proxyPort := 9090
// 	numClients := 1 // Number of clients to create

// 	clients := make([]safeClient, numClients)
// 	for i := 0; i < numClients; i++ {
// 		client := &ProxyClient{}
// 		err := client.Init(proxyAddress, proxyPort)
// 		if err != nil {
// 			log.Fatalf("Couldn't Connect to Executor Proxy for client %d. Error: %s", i, err)
// 		}
// 		clients[i] = safeClient{client: client}
// 	}

// 	Keys, Values := generateDataSet()

// 	// Initialize DB only for the first client
// 	clients[0].client.InitArgs(10, 8, 2, 100, 1, 1)
// 	clients[0].client.InitDB(Keys, Values)

// 	var wg sync.WaitGroup
// 	rand.Seed(uint64(time.Now().UnixNano()))

// 	for _, tc := range testCases {
// 		t.Run(tc.name, func(t *testing.T) {
// 			fmt.Printf("Running test: %s \n", tc.name)

// 			wg.Add(1)
// 			go func(tc TestCase) {
// 				defer wg.Done()

// 				// Find an available client
// 				var selectedClient *safeClient
// 				for {
// 					clientIndex := rand.Intn(numClients)
// 					if clients[clientIndex].mutex.TryLock() {
// 						selectedClient = &clients[clientIndex]
// 						break
// 					}
// 					// If no client is available, wait a bit and try again
// 					time.Sleep(10 * time.Millisecond)
// 				}
// 				defer selectedClient.mutex.Unlock()

// 				// Call the MixBatch function with the test case Keys and Values
// 				resp, err := selectedClient.client.MixBatch(tc.Keys, tc.Values)
// 				if err != nil {
// 					t.Errorf("MixBatch Error for client = %v", err)
// 					return
// 				}

// 				// Compare the actual results with the expected results
// 				if !reflect.DeepEqual(resp, tc.expected) {
// 					fmt.Println(tc.expected, resp)
// 					t.Errorf("ExecuteBatch mismatch! \nExpected: %v, \nGot: %v \n", tc.expected, resp)
// 				}
// 			}(tc)
// 		})
// 	}

// 	// Wait for all goroutines to finish
// 	wg.Wait()
// }
