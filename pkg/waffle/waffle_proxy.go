package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	_ "net/http/pprof"
	"sync"
	"sync/atomic"
	"time"

	waffle_service "github.com/Haseeb1399/waffle-go/api/waffle"
	"github.com/redis/go-redis/v9"
	"golang.org/x/exp/rand"
	"google.golang.org/grpc"
)

type OperationType int

type WaffleProxy struct {
	waffle_service.UnimplementedKeyValueStoreServer

	operationQueue    chan *Operation
	outputLocation    string
	traceLocation     string
	serverHostName    string
	serverPort        int32
	securityBatchSize int32
	objectSize        int32
	keySize           int32
	redisBulkLength   int32
	serverCount       int32
	serverType        string
	pThreads          int32
	storageBatchSize  int32
	core              int32
	isStatic          bool
	dateString        string
	latency           bool
	ticksPerNs        float64
	R                 int32
	B                 int32
	F                 int32
	D                 int32
	cacheBatches      int32
	keyValueMap       map[string]string
	keysNotUsed       []string
	numCores          int32
	timeStamp         atomic.Int32
	realBst           *FrequencySmoother
	fakeBst           *FrequencySmoother
	encryptionEngine  *EncryptionEngine
	cache             *Cache
	evictedItems      *EvictedItems
	storageInterface  *redis.Client
	operationQueues   []chan Operation
	finished          bool
	mutex             sync.Mutex
	ctx               context.Context
	cancelFunc        context.CancelFunc
	runningKeys       *ThreadSafeUnorderedMap
}

func redisKeyValuePairs(keys []string, values []string) []interface{} {
	pairs := make([]interface{}, 0, len(keys)*2)
	for i := 0; i < len(keys); i++ {
		pairs = append(pairs, keys[i], values[i])
	}
	return pairs
}

func NewWaffleProxy(port, R, F, D, B, C, N int32, serverHostName string) *WaffleProxy {
	ctx, cancelFunc := context.WithCancel(context.Background())
	return &WaffleProxy{
		operationQueue:    make(chan *Operation, R),
		outputLocation:    "log",
		serverHostName:    serverHostName,
		serverPort:        port,
		securityBatchSize: 3,
		objectSize:        1024,
		keySize:           16,
		serverCount:       1,
		serverType:        "redis",
		pThreads:          20,
		storageBatchSize:  B,
		isStatic:          true,
		R:                 R,
		B:                 B,
		F:                 F,
		D:                 D,
		cacheBatches:      C,
		keyValueMap:       make(map[string]string),
		numCores:          N,
		redisBulkLength:   524287,
		ctx:               ctx,
		cancelFunc:        cancelFunc,
		runningKeys:       NewThreadSafeUnorderedMap(),
		timeStamp:         atomic.Int32{},
		keysNotUsed:       []string{},
	}
}

func (w *WaffleProxy) BatchInsertToRedis() {
	finalredisKeys := []string{}
	finalredisValues := []string{}

	for k, v := range w.keyValueMap {
		finalredisKeys = append(finalredisKeys, k)
		finalredisValues = append(finalredisValues, v)

		// Check if we have reached the batch size limit
		if len(finalredisValues) == int(w.redisBulkLength) {
			// Prepare arguments for MSet
			args := make([]interface{}, 0, len(finalredisKeys)*2)
			for i, key := range finalredisKeys {
				args = append(args, key, finalredisValues[i])
			}

			// Perform batch insert into Redis
			err := w.storageInterface.MSet(context.Background(), args...).Err()
			if err != nil {
				log.Printf("Failed to insert batch into Redis: %v", err)
			}

			// Clear the slices for the next batch
			finalredisKeys = []string{}
			finalredisValues = []string{}
		}
	}

	// Insert remaining key-value pairs if any
	if len(finalredisValues) > 0 {
		args := make([]interface{}, 0, len(finalredisKeys)*2)
		for i, key := range finalredisKeys {
			args = append(args, key, finalredisValues[i])
		}

		err := w.storageInterface.MSet(context.Background(), args...).Err()
		if err != nil {
			log.Printf("Failed to insert final batch into Redis: %v", err)
		}
	}
}

func (w *WaffleProxy) Init(ctx context.Context, req *waffle_service.InitRequest) (*waffle_service.Empty, error) {
	w.realBst = NewFrequencySmoother()
	w.fakeBst = NewFrequencySmoother()
	w.encryptionEngine = NewEncryptionEngine()
	keysCacheUnencrypted := []string{}
	allKeys := make(map[string]struct{})
	tempFakeKeys := make(map[string]struct{})

	if w.serverType == "redis" {
		redisAddr := w.serverHostName + ":" + fmt.Sprint(w.serverPort)
		rdb := redis.NewClient(&redis.Options{
			Addr: redisAddr,
		})
		w.storageInterface = rdb
		fmt.Println("Storage interface is initialized with Redis DB")
	}
	// fmt.Println("Max Cores is: %s", runtime.NumCPU())
	// runtime.GOMAXPROCS(runtime.NumCPU()) //Default Max Number of cores.

	fmt.Println("Key Size in init() is:", len(req.Keys))
	//Adding the Data to the Database
	for i, k := range req.Keys {
		w.realBst.Insert(k)

		keyEncrypted := w.encryptionEngine.PRF(fmt.Sprintf("%s#%d", k, w.realBst.GetFrequency(k)))
		valEncrypted, err := w.encryptionEngine.EncryptNonDeterministic(req.Values[i])
		if err != nil {
			log.Fatalf("Failed to encrypted value in Init!")
		}

		w.keyValueMap[keyEncrypted] = valEncrypted
	}
	//Initialising Cache
	cacheCapacity := math.Ceil(float64(w.cacheBatches*int32(len(req.Keys))) / 100)

	temp := make(map[string]struct{})
	valuesCache := []string{}

	for len(keysCacheUnencrypted) < int(cacheCapacity) {
		index := rand.Intn(len(req.Keys))
		if _, found := temp[req.Keys[index]]; !found {
			temp[req.Keys[index]] = struct{}{}
			keysCacheUnencrypted = append(keysCacheUnencrypted, req.Keys[index])
			valuesCache = append(valuesCache, req.Values[index])
			key := w.encryptionEngine.PRF(fmt.Sprintf("%s#%d", req.Keys[index], w.realBst.GetFrequency(req.Keys[index])))

			if _, found := w.keyValueMap[key]; !found {
				fmt.Println("Warning: Key is Missing and this should not happen")
			}
			delete(w.keyValueMap, key)
		}
	}

	w.cache = NewCache(keysCacheUnencrypted, valuesCache, int(cacheCapacity)*10)
	w.evictedItems = NewEvictedItems()
	for i := 0; i < int(w.D); {
		fakeKey := genRandom(rand.Intn(10))
		if _, found := allKeys[fakeKey]; !found {
			if _, found := tempFakeKeys[fakeKey]; !found {
				i++
				w.fakeBst.Insert(fakeKey)
				tempFakeKeys[fakeKey] = struct{}{}
				key := w.encryptionEngine.PRF(fakeKey + "#" + fmt.Sprintf("%d", w.fakeBst.GetFrequency(fakeKey)))
				value, _ := w.encryptionEngine.EncryptNonDeterministic(genRandom(1 + rand.Intn(10)))
				w.keyValueMap[key] = value
			}
		}
	}
	w.BatchInsertToRedis()

	//Tests for Encryption
	fmt.Println("##################################")
	test1, _ := w.encryptionEngine.EncryptNonDeterministic("test")
	test2, _ := w.encryptionEngine.EncryptNonDeterministic("test")
	if test1 == test2 {
		fmt.Println("Non Deterministic Encryption is same")
	} else {
		fmt.Println("Non Deterministic Encryption is not same")
	}

	test3, _ := w.encryptionEngine.DecryptNonDeterministic(test1)
	test4, _ := w.encryptionEngine.DecryptNonDeterministic(test2)

	if test3 == test4 {
		fmt.Println("Non deterministic Decryption is same and the value is", test3)
	} else {
		fmt.Println("Non Deterministic Encryption is not same", test3)
		fmt.Println("Non Deterministic Encryption is not same", test4)
	}

	test5 := w.encryptionEngine.PRF("Testing123")
	test6 := w.encryptionEngine.PRF("Testing123")

	if test5 == test6 {
		fmt.Println("PRF encryption is same")
	}
	fmt.Println("##################################")
	//Sizes of cipher texts = 64 or greater.
	//Clear the local map
	for k := range w.keyValueMap {
		delete(w.keyValueMap, k)
	}

	go w.consumer_thread()
	go w.clearThread()

	return &waffle_service.Empty{}, nil
}

func (w *WaffleProxy) Get(ctx context.Context, req *waffle_service.GetRequest) (*waffle_service.GetResponse, error) {
	key := req.Key

	// Create a result channel to receive the result from the consumer thread
	resultChan := make(chan string, 1)

	// Create a GET operation
	op := &Operation{
		Key:    key,
		Value:  "",
		Result: resultChan,
	}
	select {
	case w.operationQueue <- op:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	// Wait for the result from the consumer thread
	select {
	case result := <-resultChan:
		return &waffle_service.GetResponse{Value: result}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (w *WaffleProxy) Put(ctx context.Context, req *waffle_service.PutRequest) (*waffle_service.GetResponse, error) {
	key := req.Key
	val := req.Value
	// Create a result channel to receive the result from the consumer thread
	resultChan := make(chan string, 1)

	// Create a GET operation
	op := &Operation{
		Key:    key,
		Value:  val,
		Result: resultChan,
	}

	select {
	case w.operationQueue <- op:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	// Wait for the result from the consumer thread
	select {
	case result := <-resultChan:
		return &waffle_service.GetResponse{Value: result}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (w *WaffleProxy) GetBatch(ctx context.Context, req *waffle_service.GetBatchRequest) (*waffle_service.GetBatchResponse, error) {
	// Prepare a slice to hold result channels for each operation
	resultChans := make([]chan string, len(req.Keys))

	// Prepare a slice to hold the responses
	results := make([]string, len(req.Keys))

	// Create and push a GET operation for each key
	for i, key := range req.Keys {
		// Create a result channel for the current operation
		resultChan := make(chan string, 1)
		resultChans[i] = resultChan

		// Create a GET operation
		op := &Operation{
			Key:    key,
			Value:  "",
			Result: resultChan,
		}

		// Send the operation to the operationQueue
		select {
		case w.operationQueue <- op:
		case <-ctx.Done():
			// If the context is done, return the error
			return nil, ctx.Err()
		}
	}

	// Collect results from all result channels
	for i, resultChan := range resultChans {
		select {
		case result := <-resultChan:
			results[i] = result
		case <-ctx.Done():
			// If the context is done while waiting for results, return the error
			return nil, ctx.Err()
		}
	}

	// Prepare the batch response
	response := &waffle_service.GetBatchResponse{
		Values: results,
	}

	return response, nil

}

func (w *WaffleProxy) PutBatch(ctx context.Context, req *waffle_service.PutBatchRequest) (*waffle_service.GetBatchResponse, error) {
	resultChans := make([]chan string, len(req.Keys))

	// Prepare a slice to hold the responses
	results := make([]string, len(req.Keys))

	// Create and push a GET operation for each key
	for i, key := range req.Keys {
		// Create a result channel for the current operation
		resultChan := make(chan string, 1)
		resultChans[i] = resultChan

		// Create a GET operation
		op := &Operation{
			Key:    key,
			Value:  req.Values[i],
			Result: resultChan,
		}

		// Send the operation to the operationQueue
		select {
		case w.operationQueue <- op:
		case <-ctx.Done():
			// If the context is done, return the error
			return nil, ctx.Err()
		}
	}

	// Collect results from all result channels
	for i, resultChan := range resultChans {
		select {
		case result := <-resultChan:
			results[i] = result
		case <-ctx.Done():
			// If the context is done while waiting for results, return the error
			return nil, ctx.Err()
		}
	}

	// Prepare the batch response
	response := &waffle_service.GetBatchResponse{
		Values: results,
	}

	return response, nil
}

func (w *WaffleProxy) ExecuteMixBatch(ctx context.Context, req *waffle_service.PutBatchRequest) (*waffle_service.ExecuteBatchResponse, error) {
	resultChans := make([]chan string, len(req.Keys))

	// Prepare slices to hold the responses
	results := make([]string, len(req.Keys))
	keys := make([]string, len(req.Keys))

	// Create and push a GET operation for each key
	for i, key := range req.Keys {
		// Create a result channel for the current operation
		resultChan := make(chan string, 1)
		resultChans[i] = resultChan

		// Create a GET operation
		op := &Operation{
			Key:    key,
			Value:  req.Values[i],
			Result: resultChan,
		}

		// Send the operation to the operationQueue
		select {
		case w.operationQueue <- op:
		case <-ctx.Done():
			// If the context is done, return the error
			return nil, ctx.Err()
		}

		// Store the key for response
		keys[i] = key
	}

	// Collect results from all result channels
	for i, resultChan := range resultChans {
		select {
		case result := <-resultChan:
			results[i] = result
		case <-ctx.Done():
			// If the context is done while waiting for results, return the error
			return nil, ctx.Err()
		}
	}

	// Prepare the batch response
	response := &waffle_service.ExecuteBatchResponse{
		Keys:   keys,
		Values: results,
	}

	return response, nil
}

func (wp *WaffleProxy) create_security_batch(op *Operation, storageBatch *[]Operation, keyToPromiseMap map[string][]chan string, cacheMisses *int) {
	wp.mutex.Lock()
	defer wp.mutex.Unlock()

	currentKey := op.Key
	currentValue := op.Value
	isPresentInCache := false
	val, isPresentInCache := wp.cache.GetValueWithoutPositionChangeNew(currentKey)
	isPresentInTree := wp.realBst.GetFrequency(currentKey)
	valEvicted := wp.evictedItems.GetValue(currentKey)

	if currentValue == "" {
		//It's a GET Request
		if isPresentInTree == -1 {
			op.Result <- "-1" //Case for Handling Missing Keys
		} else if isPresentInCache {
			op.Result <- val
		} else if valEvicted != "" {
			op.Result <- valEvicted
		} else {
			// Key is not in cache or evicted items, so we need to fetch it from the storage
			isPresentInRunningKeys := wp.runningKeys.InsertIfNotPresent(currentKey, op.Result)
			if !isPresentInRunningKeys {
				*storageBatch = append(*storageBatch, *op)
			}
			(*cacheMisses)++
		}
	} else if currentKey != "" {
		// It's a PUT request
		if !wp.cache.CheckIfKeyExists(currentKey) && !wp.evictedItems.CheckIfKeyExists(currentKey) {
			isPresentInRunningKeys := wp.runningKeys.InsertIfNotPresent(currentKey, op.Result)
			if !isPresentInRunningKeys {
				*storageBatch = append(*storageBatch, *op)
				if isPresentInTree == -1 {
					wp.realBst.Insert(currentKey)
				}
			}
			(*cacheMisses)++
		}
		wp.cache.InsertIntoCache(currentKey, currentValue)
		op.Result <- wp.cache.GetValueWithoutPositionChange(currentKey)
	}
}

func (wp *WaffleProxy) execute_batch(operations []Operation, keyToPromiseMap map[string][]chan string, storageInterface *redis.Client, encEngine *EncryptionEngine, cacheMisses *int) {
	wp.mutex.Lock()
	defer wp.mutex.Unlock()

	storageKeys := []string{}
	writeBatchKeys := []string{}
	writeBatchValues := []string{}
	readBatchMap := make(map[string]string)
	timeStamp := wp.timeStamp.Add(1)

	// Initialize the storage batch
	for _, op := range operations {
		key := op.Key
		stKey := encEngine.PRF(fmt.Sprintf("%s#%d", key, wp.realBst.GetFrequency(key)))
		readBatchMap[stKey] = key
		storageKeys = append(storageKeys, stKey)
		wp.realBst.SetFrequency(key, int(timeStamp))
	}

	realKeysNotInCache := []string{}
	i := 0
	for _, kv := range wp.realBst.accessTree {
		if i >= int(wp.B)-(int(len(operations))+int(wp.F)) {
			break
		}

		if !wp.cache.CheckIfKeyExists(kv.Key) && !wp.evictedItems.CheckIfKeyExists(kv.Key) {
			isPresentInRunningKeys := wp.runningKeys.InsertIfNotPresent(kv.Key, nil)
			if !isPresentInRunningKeys {
				realKeysNotInCache = append(realKeysNotInCache, kv.Key)
				i++
			}
		}
	}

	for _, key := range realKeysNotInCache {
		stKey := encEngine.PRF(fmt.Sprintf("%s#%d", key, wp.realBst.GetFrequency(key)))
		readBatchMap[stKey] = key
		storageKeys = append(storageKeys, stKey)
		wp.realBst.SetFrequency(key, int(timeStamp))
	}

	for i := 0; i < int(wp.F); i++ {
		fakeMinKey := wp.fakeBst.GetKeyWithMinFrequency()
		isPresentInRunningKeys := wp.runningKeys.InsertIfNotPresent(fakeMinKey, nil)
		if !isPresentInRunningKeys {
			stKey := encEngine.PRF(fmt.Sprintf("%s#%d", fakeMinKey, wp.fakeBst.GetFrequency(fakeMinKey)))
			readBatchMap[stKey] = fakeMinKey
			storageKeys = append(storageKeys, stKey)
			wp.fakeBst.SetFrequency(fakeMinKey, int(timeStamp))
		}
	}

	// Fetch data from storage (Redis)
	responses, err := wp.storageInterface.MGet(wp.ctx, storageKeys...).Result()
	if err != nil {
		log.Fatalf("Error fetching from Redis: %v", err)
	}
	tempEvictedItems := []string{}
	promiseSatisfy := make(map[string]string)
	// Process responses

	for i, stKey := range storageKeys {
		if i < len(operations)+len(realKeysNotInCache) {
			// Handle real keys
			var kvPair []string
			key, value := wp.cache.EvictLRElementFromCache()
			kvPair = []string{key, value}

			wp.evictedItems.Insert(kvPair[0], kvPair[1])
			tempEvictedItems = append(tempEvictedItems, kvPair[0])
			writeBatchKey := encEngine.PRF(fmt.Sprintf("%s#%d", kvPair[0], wp.realBst.GetFrequency(kvPair[0])))
			writeBatchValue, err := encEngine.EncryptNonDeterministic(kvPair[1])
			if err != nil {
				log.Fatalf("Error encrypting value: %v", err)
			}
			writeBatchKeys = append(writeBatchKeys, writeBatchKey)
			writeBatchValues = append(writeBatchValues, writeBatchValue)

			keyAboutToGoToCache := readBatchMap[stKey]
			valueAboutToGoToCache, err := encEngine.DecryptNonDeterministic(responses[i].(string))
			if err != nil {
				log.Fatalf("Error decrypting value: %v", err)
			}

			if wp.cache.CheckIfKeyExists(keyAboutToGoToCache) {
				valueAboutToGoToCache = wp.cache.GetValueWithoutPositionChange(keyAboutToGoToCache)
			}

			if key == keyAboutToGoToCache {
				valueAboutToGoToCache = value
			}
			promiseSatisfy[keyAboutToGoToCache] = valueAboutToGoToCache
			wp.cache.InsertIntoCache(keyAboutToGoToCache, valueAboutToGoToCache)

		} else {
			// Handle fake keys
			fakeWriteKey := readBatchMap[stKey]
			decryptedValue, err := encEngine.DecryptNonDeterministic(responses[i].(string))
			if err != nil {
				log.Fatalf("Error decrypting fake value: %v", err)
			}
			promiseSatisfy[fakeWriteKey] = decryptedValue

			writeBatchKey := encEngine.PRF(fmt.Sprintf("%s#%d", fakeWriteKey, wp.fakeBst.GetFrequency(fakeWriteKey)))
			fakeWriteValue, err := encEngine.EncryptNonDeterministic(genRandom(1 + rand.Intn(10)))
			if err != nil {
				log.Fatalf("Error encrypting fake value: %v", err)
			}
			writeBatchKeys = append(writeBatchKeys, writeBatchKey)
			writeBatchValues = append(writeBatchValues, fakeWriteValue)
		}
	}
	// Write data back to storage
	err = wp.storageInterface.MSet(wp.ctx, redisKeyValuePairs(writeBatchKeys, writeBatchValues)...).Err()
	if err != nil {
		log.Fatalf("Error writing batch to Redis: %v", err)
	}

	// Remove temporarily evicted items
	for _, it := range tempEvictedItems {
		wp.evictedItems.Erase(it)
	}

	for key, value := range promiseSatisfy {
		wp.runningKeys.ClearPromises(key, value)
	}
	wp.keysNotUsed = append(wp.keysNotUsed, storageKeys...)
}

func (wp *WaffleProxy) consumer_thread() {
	fmt.Println("Entering Consumer Thread")
	for !wp.finished {
		storageBatch := []Operation{}
		keyToPromiseMap := make(map[string][]chan string)
		cacheMisses := 0
		i := 0

		for int32(i) < wp.R && !wp.finished {
			select {
			case op := <-wp.operationQueue:
				wp.create_security_batch(op, &storageBatch, keyToPromiseMap, &cacheMisses)
				i++
			case <-time.After(1 * time.Millisecond):
				// If no operation is received within 10ms, continue waiting
				continue
			}
		}
		wp.execute_batch(storageBatch, keyToPromiseMap, wp.storageInterface, wp.encryptionEngine, &cacheMisses)
	}

}

func (wp *WaffleProxy) clearThread() {
	// Create a ticker that ticks every second
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-wp.ctx.Done():
			// Context canceled, exit the loop
			return
		case <-ticker.C:
			// Ticker has ticked, perform the deletion operation
			if len(wp.keysNotUsed) > 0 {
				// Deleting batch of keys from Redis
				wp.mutex.Lock()
				err := wp.storageInterface.Del(wp.ctx, wp.keysNotUsed...).Err()
				if err != nil {
					log.Printf("Error deleting batch from Redis: %v", err)
					log.Printf("Keys not deleted: %v", wp.keysNotUsed)
				} else {
					log.Printf("Successfully deleted batch of keys of length: %d", len(wp.keysNotUsed))
				}
				// Clear the keysNotUsed slice after deletion
				wp.keysNotUsed = []string{}
				wp.mutex.Unlock()
			}
		}
	}
}

func (w *WaffleProxy) Close(context.Context, *waffle_service.Empty) (*waffle_service.Empty, error) {
	w.finished = true
	w.ctx.Done()
	return &waffle_service.Empty{}, nil
}

func main() {
	rand.Seed(uint64(time.Now().UnixNano()))
	// Define command-line flags
	port := flag.Int("p", 6379, "Storage server port")
	R := flag.Int("r", 50, "System parameter R")
	F := flag.Int("f", 5, "Number of fake queries for dummy objects")
	D := flag.Int("d", 10, "Number of dummy objects")
	B := flag.Int("b", 5, "Batch size")
	C := flag.Int("c", 2, "Cache size")
	N := flag.Int("n", 1, "Number of cores")
	host := flag.String("h", "localhost", "Storage server IP")

	// Parse command-line flags
	flag.Parse()

	// Initialize the server with provided command-line arguments
	service := NewWaffleProxy(int32(*port), int32(*R), int32(*F), int32(*D), int32(*B), int32(*C), int32(*N), *host)

	lis, err := net.Listen("tcp", ":9090")
	if err != nil {
		log.Fatalf("Cannot create listener on port :9090 %s", err)

	}
	fmt.Println("Starting Waffle Proxy on: localhost:9090")

	serverRegister := grpc.NewServer(grpc.MaxRecvMsgSize(644000*300), grpc.MaxSendMsgSize(644000*300))
	waffle_service.RegisterKeyValueStoreServer(serverRegister, service)

	fmt.Println("R:", service.R, "B:", service.B)
	//Start serving
	go func() {
		http.ListenAndServe(":10001", nil)
	}()
	err = serverRegister.Serve(lis)
	if err != nil {
		log.Fatalf("Error! Could not start loadBalancer! %s", err)
	}

}
