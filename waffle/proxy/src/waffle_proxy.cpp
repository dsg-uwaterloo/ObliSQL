#include "waffle_proxy.h"
#include <cstddef>
#include <fstream>
#include <string>
#include <unistd.h>


// #include "opentelemetry/exporters/otlp/otlp_grpc_exporter_factory.h"
// #include "opentelemetry/exporters/otlp/otlp_grpc_exporter_options.h"
// #include "opentelemetry/sdk/trace/processor.h"
// #include "opentelemetry/sdk/trace/batch_span_processor_factory.h"
// #include "opentelemetry/sdk/trace/batch_span_processor_options.h"
// #include "opentelemetry/sdk/trace/tracer_provider_factory.h"
// #include "opentelemetry/trace/provider.h"
// #include "opentelemetry/sdk/trace/tracer_provider.h"

// #include "opentelemetry/exporters/otlp/otlp_grpc_metric_exporter_factory.h"
// #include "opentelemetry/exporters/otlp/otlp_grpc_metric_exporter_options.h"
// #include "opentelemetry/metrics/provider.h"
// #include "opentelemetry/sdk/metrics/aggregation/default_aggregation.h"
// #include "opentelemetry/sdk/metrics/export/periodic_exporting_metric_reader.h"
// #include "opentelemetry/sdk/metrics/export/periodic_exporting_metric_reader_factory.h"


// namespace trace_api = opentelemetry::trace;
// namespace trace_sdk = opentelemetry::sdk::trace;

// namespace otlp = opentelemetry::exporter::otlp;

// void InitTracer()
// {
//   trace_sdk::BatchSpanProcessorOptions bspOpts{};
//   opentelemetry::exporter::otlp::OtlpGrpcExporterOptions opts;
//   opts.endpoint = "localhost:4317";
//   opts.use_ssl_credentials = false;
//   auto exporter  = otlp::OtlpGrpcExporterFactory::Create(opts);
//   auto processor = trace_sdk::BatchSpanProcessorFactory::Create(std::move(exporter), bspOpts);
//   std::shared_ptr<trace_api::TracerProvider> provider =
//       trace_sdk::TracerProviderFactory::Create(std::move(processor));
//   // Set the global trace provider
//   trace_api::Provider::SetTracerProvider(provider);
// }

// void CleanupTracer() {
//     std::shared_ptr<opentelemetry::trace::TracerProvider> none;
//     trace_api::Provider::SetTracerProvider(none);
// }

void randomize_map(const std::unordered_map<std::string, std::string>& input_map, std::vector<std::string>& keys, std::vector<std::string>& values) {
    for (const auto& it : input_map) {
        keys.push_back(it.first);
        values.push_back(it.second);
    }
    // std::random_device rd;
    // std::mt19937 g(rd());
    // std::shuffle(keys.begin(), keys.end(), g);
    // std::shuffle(values.begin(), values.end(), g);
}

#define rdtscllProxy(val) do { \
    unsigned int __a,__d; \
    __asm__ __volatile__("rdtsc" : "=a" (__a), "=d" (__d)); \
    (val) = ((unsigned long long)__a) | (((unsigned long long)__d)<<32); \
} while(0)


int _mkdirProxy(const char *path) {
    #ifdef _WIN32
        return ::_mkdirProxy(path);
    #else
        #if _POSIX_C_SOURCE
            return ::mkdir(path, 0755);
        #else
            return ::mkdir(path, 0755); // not sure if this works on mac
        #endif
    #endif
}

std::string extractKey(std::string encryptedKey) {
    for(int i=encryptedKey.size()-1; i>=0; --i) {
        if(encryptedKey[i] == '#') {
            return encryptedKey.substr(0, i);
        }
    }
    return "";
}

std::string gen_random(const int len) {
    static const char alphanum[] =
        "0123456789"
        "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
        "abcdefghijklmnopqrstuvwxyz";
    std::string tmp_s;
    tmp_s.reserve(len);

    for (int i = 0; i < len; ++i) {
        tmp_s += alphanum[rand() % (sizeof(alphanum) - 1)];
    }
    
    return tmp_s;
}


unsigned long long rdtscuhzProxy(void) {
    const int ntrials = 5;
    const long sleeplen = (250 * 1000); // in us
    double freq = 0.0;

    for (int i = 0; i < ntrials; i++) {
        unsigned long long start, end, t;
        double hz;

        auto s = std::chrono::high_resolution_clock::now();
        rdtscllProxy(start);
        usleep(sleeplen);
        rdtscllProxy(end);
        auto e = std::chrono::high_resolution_clock::now();
        t = std::chrono::duration_cast<std::chrono::microseconds>(e - s).count();
        hz = ((double)(end - start))/t;
        freq += hz;
    }

    freq = round(freq/ntrials);
    return (unsigned long long) freq;
}

void waffle_proxy::init_map(void **args){
    id_to_client_ = *(static_cast<std::shared_ptr<thrift_response_client_map>*>(args[0]));
    
}

int get_current_timestamp() {
    auto now = std::chrono::system_clock::now();
    int timestamp = std::chrono::duration_cast<std::chrono::milliseconds>(now.time_since_epoch()).count();
    return timestamp;
}

int calculate_time_difference(int start_timestamp, int end_timestamp) {
    return end_timestamp - start_timestamp; // The difference in milliseconds
}


void waffle_proxy::init_args(const int64_t B, const int64_t R, const int64_t F, const int64_t D, const int64_t C, const int64_t N){
    this->B=B;
    this->R=R;
    this->F=F;
    this->D=D;
    this->cacheBatches=C;
    this->num_cores=N;
    
    std::cout<<"Arguments Initialized \n";
}

void waffle_proxy::init(const std::vector<std::string> &keys, const std::vector<std::string> &values, void ** args){
    std::cout<<"Init Called with Key Size: "<<keys.size()<<std::endl;
    // InitTracer();
    // CleanupTracer();
    std::cout<<keys.size()<<":"<<values.size()<<std::endl;
    std::unordered_set<std::string> allKeys;
    std::unordered_set<std::string> tempFakeKeys;
    realBst = FrequencySmoother();
    fakeBst = FrequencySmoother();
    std::vector<std::string> keysCache;
    std::vector<std::string> keysCacheUnencrypted;
    
    id_to_client_ = *(static_cast<std::shared_ptr<thrift_response_client_map>*>(args[0]));

    if (server_type_ == "redis") {
        storage_interface_ = std::make_shared<redis>(server_host_name_, server_port_);
        cpp_redis::network::set_default_nb_workers(std::min(10, p_threads_));
        std::cout << "Storage interface is initialized with Redis DB " << std::endl;
    }

    for (int i = 1; i < server_count_; i++) {
        storage_interface_->add_server(server_host_name_, server_port_+i);
    }


    //int num_cores = sysconf(_SC_NPROCESSORS_ONLN);
    std::cout << "max cores is " << sysconf(_SC_NPROCESSORS_ONLN) << std::endl << " and current cores used is " << num_cores;
    std::vector<std::thread> threads;
    for (int i = 0; i < num_cores; i++) {
        auto q = std::make_shared<queue<std::pair<operation, std::shared_ptr<std::promise<std::string>>>>>();
        operation_queues_.push_back(q);
    }
    for (int i = 0; i < num_cores; i++) {
        threads_.push_back(std::thread(&waffle_proxy::consumer_thread, this, i, new encryption_engine(encryption_engine_)));
    }
    threads_.push_back(std::thread(&waffle_proxy::responder_thread, this));
    
    //Adding the data to Database
    std::cout << "Keys size in init() is " << keys.size() << std::endl;
    for(int i = 0; i<keys.size(); ++i) {
        if(values[i]==""){
            std::cout<<keys[i]<<std::endl;
        }
        realBst.insert(keys[i]);
        keyValueMap[encryption_engine_.prf(keys[i] + "#" + std::to_string(realBst.getFrequency(keys[i])))] = encryption_engine_.encryptNonDeterministic(values[i]);
    }

    // Initialising Cache
    size_t cacheCapacity = cacheBatches*keys.size()/100;
    std::unordered_set<std::string> temp;
    std::vector<std::string> valuesCache;
    while(keysCacheUnencrypted.size() < cacheCapacity) {
        auto index = rand()%(keys.size());
        if(temp.find(keys[index]) == temp.end()) {
            temp.insert(keys[index]);
            keysCacheUnencrypted.push_back(keys[index]);
            valuesCache.push_back(values[index]);
            if(keyValueMap.find(encryption_engine_.prf(keys[index] + "#" + std::to_string(realBst.getFrequency(keys[index])))) == keyValueMap.end()) {
                std::cout << "WARNING: Key is Missing & this should not happen" << std::endl;
            }
            keyValueMap.erase(encryption_engine_.prf(keys[index] + "#" + std::to_string(realBst.getFrequency(keys[index]))));
        }
    }

    cache = Cache(keysCacheUnencrypted, valuesCache, cacheCapacity*10);
    EvictedItems = evictedItems();

    for(int i=0; i < D; ) {
        auto fakeKey = gen_random(rand()%10);
        if(allKeys.find(fakeKey) == allKeys.end() && tempFakeKeys.find(fakeKey)==tempFakeKeys.end()) {
            ++i;
            fakeBst.insert(fakeKey);
            tempFakeKeys.insert(fakeKey);
            auto tempFakeKey = encryption_engine_.prf(fakeKey + "#" + std::to_string(fakeBst.getFrequency(fakeKey)));
            auto fakeKeyValue = encryption_engine_.encryptNonDeterministic(gen_random(1 + rand()%10));
            keyValueMap[tempFakeKey] = fakeKeyValue;
        }
    }

    std::vector<std::string> redisKeys;
    std::vector<std::string> redisValues;
    randomize_map(keyValueMap, redisKeys, redisValues);

    std::vector<std::string> finalKeysRedis;
    std::vector<std::string> finalValuesRedis;
    for(int i=0;i<redisKeys.size();++i) {
        finalKeysRedis.push_back(redisKeys[i]);
        finalValuesRedis.push_back(redisValues[i]);
        if(finalValuesRedis.size() == redisBulkLength) {
            storage_interface_->put_batch(finalKeysRedis, finalValuesRedis);
            finalValuesRedis.clear();
            finalKeysRedis.clear();
        }
    }
    if(finalValuesRedis.size() > 0) { 
        storage_interface_->put_batch(finalKeysRedis, finalValuesRedis);
    }
    threads_.push_back(std::thread(&waffle_proxy::clearThread, this));

    //Tests for the encryption
    std::cout << "##################################" << std::endl;
    auto test3 = encryption_engine_.encryptNonDeterministic("mynameissharath");
    auto test4 = encryption_engine_.encryptNonDeterministic("mynameissharath");
    if(test3 == test4) {
        std::cout << "Non Deterministic Encryption is same" << std::endl;
    } else {
        std::cout << "Non Deterministic Encryption is not same" << std::endl;
    }

    std::string test5 = encryption_engine_.decryptNonDeterministic(test3);
    std::string test6 = encryption_engine_.decryptNonDeterministic(test4);
    if(test5 == test6) {
        std::cout << "non deterministic Decryption is same and the value is " << test5 << std::endl;
    } else {
        std::cout << "non deterministic Decryption is not same and the value is " << test5 << std::endl;
        std::cout << "non deterministic Decryption is not same and the value is " << test6 << std::endl;
    }

    auto test7 = encryption_engine_.prf("mynameissharath1");
    auto test8 = encryption_engine_.prf("mynameissharath1");
    if(test7 == test8) {
        std::cout << "PRF encryption is same" << std::endl;
    }
    std::cout << "##################################" << std::endl;
    //Tests for the encryption ended

    std::time_t end_time = std::chrono::system_clock::to_time_t(std::chrono::system_clock::now());
    date_string = std::string(std::ctime(&end_time));
    date_string = date_string.substr(0, date_string.rfind(":"));
    date_string.erase(remove(date_string.begin(), date_string.end(), ' '), date_string.end());
    if(latency) {
        std::string output_directory_bst_latency = "data/BST_Latency";
        _mkdirProxy((output_directory_bst_latency).c_str());

        std::string output_directory_redis_latency = "data/Redis_Latency";
        _mkdirProxy((output_directory_redis_latency).c_str());

        std::string output_directory_cache_miss = "data/Cache_miss";
        _mkdirProxy((output_directory_cache_miss).c_str());

        out_bst_latency = std::ofstream(output_directory_bst_latency+"/1");
        out_redis_latency = std::ofstream(output_directory_redis_latency+"/1");
        out_cache_miss = std::ofstream(output_directory_cache_miss+"/1");
    }
    
    ticks_per_ns = static_cast<double>(rdtscuhzProxy()) / 1000;
    // keyValueMap.clear();

    std::cout << "Successfully initialized waffle with keys size " << keys.size() << " and cache with " << cache.size() << " Fake keys size is " << D << " batch size is " << B << " F is" << F << " FakeBST size is " << fakeBst.size() << std::endl;
    keyValueMap.clear();
}

void waffle_proxy::create_security_batch(std::shared_ptr<queue <std::pair<operation, std::shared_ptr<std::promise<std::string>>>>> &op_queue,
                                          std::vector<operation> &storage_batch,
                                          std::unordered_map<std::string, std::vector<std::shared_ptr<std::promise<std::string>>>> &keyToPromiseMap, int& cacheMisses,int queueID) {

    if (op_queue->size() == 0) {
        std::cout << "WARNING: You should never see this line on console! Queue size is 0" << std::endl;
    } else {
        struct operation operat;
        auto operation_promise_pair = op_queue->pop();
        auto currentKey = operation_promise_pair.first.key;

        auto isPresentInRealBST = realBst.getFrequency(currentKey);
      
        if(operation_promise_pair.first.value == "") {
            // It's a GET request
            bool isPresentInCache = false;
            auto val = cache.getValueWithoutPositionChangeNew(currentKey, isPresentInCache);
            auto valEvicted = EvictedItems.getValue(currentKey);
            
            if (isPresentInRealBST == -1){
                operation_promise_pair.second->set_value("-1");
            }
            else if(isPresentInCache == true) {
                operation_promise_pair.second->set_value(val);
            }
            else if(valEvicted != "") {
                operation_promise_pair.second->set_value(valEvicted);
            } else {
                auto isPresentInRunningKeys = runningKeys.insertIfNotPresent(currentKey, operation_promise_pair.second);
                if(isPresentInRunningKeys == false) {
                    storage_batch.push_back(operation_promise_pair.first);
                }
                ++cacheMisses;
            }
        } else {
            // It's a PUT request
            if(cache.checkIfKeyExists(currentKey) == false && EvictedItems.checkIfKeyExists(currentKey) == false) {
                if(isPresentInRealBST == -1) {
                   realBst.insert(currentKey);
                }else {
                    auto isPresentInRunningKeys = runningKeys.insertIfNotPresent(currentKey);
                    if(isPresentInRunningKeys == false) {
                        storage_batch.push_back(operation_promise_pair.first);
                    }
                    ++cacheMisses;
                }

            }
            cache.insertIntoCache(currentKey, operation_promise_pair.first.value);
            operation_promise_pair.second->set_value(cache.getValueWithoutPositionChange(currentKey));
        }
    }
};

void waffle_proxy::execute_batch(const std::vector<operation> &operations, std::unordered_map<std::string, std::vector<std::shared_ptr<std::promise<std::string>>>> &keyToPromiseMap, std::shared_ptr<storage_interface> storage_interface, encryption_engine *enc_engine, int id, int& cacheMisses) {
    // Storage_keys is same as readBatch
    std::vector<std::string> storage_keys;
    std::vector<std::string> writeBatchKeys;
    std::vector<std::string> writeBatchValues;
    std::unordered_map<std::string, std::string> readBatchMap;
    uint64_t start, end;
    std::unordered_map<std::string, std::string> promiseSatisfy;
    timeStamp.fetch_add(1);

    if(latency) {
        std::string line("");
        auto cacheMissPercentage = static_cast<double>(cacheMisses)/R;
        line.append(std::to_string(cacheMissPercentage) + "\n");
        cacheMisses = 0;
        out_cache_miss << line;
        out_cache_miss.flush();
        rdtscllProxy(start);
    }
    // std::string tempName= "set_frequency"+std::to_string(id)+".log";
    // std::ofstream logFile(tempName, std::ios_base::app);
    // std::cout << "r is " << operations.size() << " f_r is " <<  B-(operations.size()+F) << std::endl;
    for(int i = 0; i < operations.size(); i++){
        std::string key = operations[i].key;
        // std::cout<<"key: "<<key<<":"<<realBst.getFrequency(key)<<std::endl;
        auto stKey = enc_engine->prf(key + "#" + std::to_string(realBst.getFrequency(key)));
        readBatchMap[stKey] = key;
        storage_keys.push_back(stKey);
        // int t1 = get_current_timestamp();
        realBst.setFrequency(key, timeStamp.load());
        // int t2 = get_current_timestamp();
        // int tdelta = calculate_time_difference(t1, t2);
        // logFile<< ", Time Difference: " << tdelta << std::endl;
    }
    // logFile.close();
    
    // std::cout<<"----------------"<<std::endl;
    std::vector<std::string> realKeysNotInCache;
    auto& bstMutex = realBst.getMutex();
    {
        std::lock_guard<std::mutex> lock(bstMutex);
        auto it = realBst.getIterator();
        for(int i=0;i<B-(operations.size()+F);) {
            if(cache.checkIfKeyExists(it->first) == false && EvictedItems.checkIfKeyExists(it->first) == false) {
                auto isPresentInRunningKeys = runningKeys.insertIfNotPresent(it->first);
                if(isPresentInRunningKeys == false) {
                    realKeysNotInCache.push_back(it->first);
                    ++i;
                }
            }
            ++it;
        }
    }

    for(auto& iter: realKeysNotInCache) {
        auto stKey = enc_engine->prf(iter + "#" + std::to_string(realBst.getFrequency(iter)));
        readBatchMap[stKey] = iter;
        storage_keys.push_back(stKey);
        realBst.setFrequency(iter, timeStamp.load());
    }

    for(int i=0;i<F;) {
        auto fakeMinKey = fakeBst.getKeyWithMinFrequency();
        auto isPresentInRunningKeys = runningKeys.insertIfNotPresent(fakeMinKey);
        if(isPresentInRunningKeys == false) {
            auto stKey = enc_engine->prf(fakeMinKey + "#" + std::to_string(fakeBst.getFrequency(fakeMinKey)));
            readBatchMap[stKey] = fakeMinKey;
            storage_keys.push_back(stKey);
            fakeBst.setFrequency(fakeMinKey, timeStamp.load());
            ++i;
        }
    }


    if (latency) {
        rdtscllProxy(end);
        double cycles = static_cast<double>(end - start);
        std::string line("");
        line.append(std::to_string((cycles / ticks_per_ns)) + "\n");
        rdtscllProxy(start);
        out_bst_latency << line;
        out_bst_latency.flush();
    }

    std::unordered_set<std::string> tempEvictedItems;
    auto responses = storage_interface->get_batch(storage_keys);
    
    for(int i = 0 ; i < storage_keys.size(); i++){
        if(i < (operations.size() + realKeysNotInCache.size())) {
            std::vector<std::string> kv_pair;
            auto& cacheMutex = cache.getMutex();
            auto& evictedItemsMutex = EvictedItems.getMutex();
            // auto& runningKeysMutex = runningKeys.getMutex();
            // This means ith request is for real key
            {
                std::lock_guard<std::mutex> lockCache(cacheMutex);
                std::lock_guard<std::mutex> lockEvicted(evictedItemsMutex);
                // std::lock_guard<std::mutex> lockRunningKeys(runningKeysMutex);
                kv_pair = cache.evictLRElementFromCache();
                EvictedItems.insert(kv_pair[0], kv_pair[1]);
            }
            tempEvictedItems.insert(kv_pair[0]);
            auto writeBatchKey = enc_engine->prf(kv_pair[0] + "#" + std::to_string(realBst.getFrequency(kv_pair[0])));
            auto writeBatchValue = enc_engine->encryptNonDeterministic(kv_pair[1]);            
            writeBatchKeys.push_back(writeBatchKey);
            writeBatchValues.push_back(writeBatchValue);
            // storage_interface_->put(writeBatchKey, writeBatchValue);

            auto keyAboutToGoToCache = readBatchMap[storage_keys[i]];
            std::string valueAboutToGoToCache = enc_engine->decryptNonDeterministic(responses[i]);
            promiseSatisfy[keyAboutToGoToCache] = valueAboutToGoToCache;
            if(cache.checkIfKeyExists(keyAboutToGoToCache) == true) {
                valueAboutToGoToCache = cache.getValueWithoutPositionChange(keyAboutToGoToCache);
            }
            cache.insertIntoCache(keyAboutToGoToCache, valueAboutToGoToCache);
        } else {
            // Writing fake key values to DB
            auto fakeWriteKey = readBatchMap[storage_keys[i]];
            promiseSatisfy[fakeWriteKey] = enc_engine->decryptNonDeterministic(responses[i]);
            writeBatchKeys.push_back(enc_engine->prf(fakeWriteKey + "#" + std::to_string(fakeBst.getFrequency(fakeWriteKey))));
            writeBatchValues.push_back(enc_engine->encryptNonDeterministic(gen_random(1 + rand()%10)));
        }
    }
    storage_interface_->put_batch(writeBatchKeys, writeBatchValues);

    for(auto& it: tempEvictedItems) {
        EvictedItems.erase(it);
    }

    for(auto& it: promiseSatisfy) {
        // std::cout << "Clearing promises " << std::endl;
        runningKeys.clearPromises(it.first, it.second);
    }
    keysNotUsed.push(storage_keys);

    if (latency) {
        rdtscllProxy(end);
        double cycles = static_cast<double>(end - start);
        std::string line("");
        line.append(std::to_string((cycles / ticks_per_ns)) + "\n");
        rdtscllProxy(start);
        out_redis_latency << line;
        out_redis_latency.flush();
    }

    // if(cache.size() != (2*B)) {
    //     std::cout << "WARNING: This should never happen: Cache size is less than 2*B" << std::endl;
    // }
};

std::string waffle_proxy::get(const std::string &key) {
    return get(rand_uint32(0, RAND_MAX), key);
};

void waffle_proxy::async_get(const sequence_id &seq_id, const std::string &key) {
    async_get(seq_id, rand_uint32(0, RAND_MAX), key);
};

void waffle_proxy::put(const std::string &key, const std::string &value) {
    return put(rand_uint32(0, RAND_MAX), key, value);
};

void waffle_proxy::async_put(const sequence_id &seq_id, const std::string &key, const std::string &value) {
    async_put(seq_id, rand_uint32(0, RAND_MAX), key, value);
};

std::vector<std::string> waffle_proxy::get_batch(const std::vector<std::string> &keys) {
    return get_batch(rand_uint32(0, RAND_MAX), keys);
};

void waffle_proxy::async_get_batch(const sequence_id &seq_id, const std::vector<std::string> &keys) {
    async_get_batch(seq_id, rand_uint32(0, RAND_MAX), keys);
};

void waffle_proxy::put_batch(const std::vector<std::string> &keys, const std::vector<std::string> &values) {
    return put_batch(rand_uint32(0, RAND_MAX), keys, values);
};

void waffle_proxy::async_put_batch(const sequence_id &seq_id, const std::vector<std::string> &keys, const std::vector<std::string> &values) {
    async_put_batch(seq_id, rand_uint32(0, RAND_MAX), keys, values);
};

std::string waffle_proxy::get(int queue_id, const std::string &key) {
    return get_future(queue_id, key).get();
};

void waffle_proxy::async_get(const sequence_id &seq_id, int queue_id, const std::string &key) {
    // Send to responder thread
    std::vector<std::pair<std::string, std::future<std::string>>> waiters;
    waiters.push_back(std::make_pair(key,get_future(queue_id, key)));

    respond_queue_.push(std::make_pair(GET, std::make_pair(seq_id, std::move(waiters))));
};

void waffle_proxy::put(int queue_id, const std::string &key, const std::string &value) {
    put_future(queue_id, key, value).get();
};

void waffle_proxy::async_put(const sequence_id &seq_id, int queue_id, const std::string &key, const std::string &value) {
    // Send to responder thread
    std::vector<std::pair<std::string, std::future<std::string>>> waiters;
    waiters.push_back(std::make_pair(key,get_future(queue_id, key)));

    respond_queue_.push(std::make_pair(GET, std::make_pair(seq_id, std::move(waiters))));
};

std::vector<std::string> waffle_proxy::get_batch(int queue_id, const std::vector<std::string> &keys) {
    std::vector<std::string> _return;
    std::vector<std::pair<std::string, std::future<std::string>>> waiters;
    for (const auto &key: keys) {
        waiters.push_back(std::make_pair(key,get_future(queue_id, key)));
    }
    for (int i = 0; i < waiters.size(); i++){
        std::string val = waiters[i].second.get();
        std::string key = waiters[i].first;
        _return.push_back(key+":"+val);
    }
    return _return;
};

void waffle_proxy::async_get_batch(const sequence_id &seq_id, int queue_id, const std::vector<std::string> &keys) {
    std::vector<std::string> _return;
    std::vector<std::pair<std::string, std::future<std::string>>> waiters;
    for (const auto &key: keys) {
        waiters.push_back(std::make_pair(key,get_future(queue_id, key)));
    }
    // std::cout << "async_get_batch client ID is " << seq_id.client_id << std::endl;
    respond_queue_.push(std::make_pair(GET_BATCH, std::make_pair(seq_id, std::move(waiters))));
    sequence_queue_.push(seq_id);
};

void waffle_proxy::put_batch(int queue_id, const std::vector<std::string> &keys, const std::vector<std::string> &values) {
    // Send waiters to responder thread
    std::vector<std::future<std::string>> waiters;
    int i = 0;
    for (const auto &key: keys) {
        waiters.push_back((put_future(queue_id, key, values[i])));
        i++;
    }
    for (int i = 0; i < waiters.size(); i++){
        waiters[i].get();
    }
};

std::vector<std::string> waffle_proxy::mix_batch(int queue_id, const std::vector<std::string> &keys, const std::vector<std::string> &values){
    // std::cout<<"Queue: "<<queue_id<<" "<<"Got Request Size:"<<keys.size()<<std::endl;
    // auto tracer = opentelemetry::trace::Provider::GetTracerProvider()->GetTracer("MixBatchRPC");
    // auto span = tracer->StartSpan("MixBatch");

    // span->SetAttribute("Num_Keys", keys.size());
    std::vector<std::string> _return;
    std::vector<std::pair<std::string, std::future<std::string>>> waiters;
    int i=0;
    // span->AddEvent("Adding to Queues");
    for (const auto &key: keys) {
        //Create a GET future
        if(values[i] == ""){
            waiters.push_back(std::make_pair(key,get_future(queue_id, key)));
        }else if(values[i] != ""){
        //Create a PUT future
            waiters.push_back(std::make_pair(key,put_future(queue_id, key, values[i])));
        }
        i++;
    }
    // span->AddEvent("Added to Queues");
    // span->AddEvent("Waiting for Promise");
    for (int i = 0; i < waiters.size(); i++){
        std::string val = waiters[i].second.get();
        std::string key = waiters[i].first;
        _return.push_back(key+":"+val);
    }
    // span->AddEvent("Waited for Promise. Sending Back Response");
    // sleep(10);
    // std::cout<<keys[0]<<"Hello"<<std::endl;
    // std::cout<<"Sending Back"<<_return.size()<<std::endl;
    return _return;
}

void waffle_proxy::async_put_batch(const sequence_id &seq_id, int queue_id, const std::vector<std::string> &keys, const std::vector<std::string> &values) {
    // Send waiters to responder thread
    std::vector<std::pair<std::string, std::future<std::string>>> waiters;
    // std::cout << "async_put_batch client ID is " << seq_id.client_id << std::endl;
    int i = 0;
    for (const auto &key: keys) {
        waiters.push_back(std::make_pair(key,get_future(queue_id, key)));
        i++;
    }

    respond_queue_.push(std::make_pair(PUT_BATCH, std::make_pair(seq_id, std::move(waiters))));
    sequence_queue_.push(seq_id);
};

std::future<std::string> waffle_proxy::get_future(int queue_id, const std::string &key) {
    auto prom = std::make_shared<std::promise<std::string>>();
    std::future<std::string> waiter = prom->get_future();
    struct operation operat;
    operat.key = key;
    operat.value = "";
    operat.enqueueTime = get_current_timestamp();
    operation_queues_[queue_id % operation_queues_.size()]->push(std::make_pair(operat, prom));
    return waiter;
};

std::future<std::string> waffle_proxy::put_future(int queue_id, const std::string &key, const std::string &value) {
    auto prom = std::make_shared<std::promise<std::string>>();
    std::future<std::string> waiter = prom->get_future();
    struct operation operat;
    operat.key = key;
    operat.value = value;
    operat.enqueueTime = get_current_timestamp();
    operation_queues_[queue_id % operation_queues_.size()]->push(std::make_pair(operat, prom));
    return waiter;
};

void waffle_proxy::consumer_thread(int id, encryption_engine *enc_engine){
    int operations_serviced = 0;
    int previous_total_operations = 0;
    int total_operations = 0;
    std::cout << "Entering here to consumer thread" << std::endl;
    // auto tracer = opentelemetry::trace::Provider::GetTracerProvider()->GetTracer("Consumer Thread");
    while (!finished_) {
        // auto span = tracer->StartSpan("ConsumerThread");

        std::vector <operation> storage_batch;
        std::unordered_map<std::string, std::vector<std::shared_ptr<std::promise<std::string>>>> keyToPromiseMap;
        //std::unordered_set<std::string> tempSet;
        int i=0;
        int cacheMisses = 0;
        // span->AddEvent("Waiting for R Requests");
        while (i < R && !finished_) {
            if(operation_queues_[id]->size() > 0) {
                create_security_batch(operation_queues_[id], storage_batch, keyToPromiseMap, cacheMisses, id);
                ++i;
            }
        }
        // span->AddEvent("Processed R requests");
        // logFile.close();
        // span->AddEvent("Executing Batch");
        execute_batch(storage_batch, keyToPromiseMap, storage_interface_, enc_engine, id, cacheMisses);
        // span->AddEvent("Executed Batch");
        // span->End();
    }

};

void waffle_proxy::responder_thread(){
    while (true){
        auto tuple = respond_queue_.pop();
        auto op_code = tuple.first;
        auto seq = tuple.second.first;
        seq = sequence_queue_.pop();
        std::vector<std::string>results;
        for (int i = 0; i < tuple.second.second.size(); i++) {
            std::string key = tuple.second.second[i].first;
            std::string val = tuple.second.second[i].second.get();
            results.push_back(key+":"+val);
        }
        id_to_client_->async_respond_client(seq, op_code, results);
    }
    std::cout << "Quitting response thread" << std::endl;
};

void waffle_proxy::clearThread(){
    while(true) {
        auto keyVectorNotUsed = keysNotUsed.pop();
        if(keyVectorNotUsed.size() > 0)
            storage_interface_->delete_batch(keyVectorNotUsed);
        
    }
}

void waffle_proxy::flush() {
    std::cout << "Flushing proxy " << std::endl;
};

void waffle_proxy::close() {
    finished_ = true;
    for (int i = 0; i < threads_.size(); i++)
        threads_[i].join();
};
