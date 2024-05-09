#include <fstream>
#include <functional>
#include <iostream>
#include <sstream>
#include <string>
#include <sys/stat.h>
#include <thread>
#include <unistd.h>
#include <unordered_map>
#include <utility>
#include <vector>
#include "timer.h"
#include "waffle_proxy.h"
#include "thrift_server.h"
#include "proxy_client.h"
#include "async_proxy_client.h"
#include "thrift_utils.h"
#define numExecutors 2


typedef std::unordered_map<int, std::vector<std::pair<std::vector<std::string>, std::vector<std::string>>>> trace_vector;
typedef std::unordered_map<int,async_proxy_client> clientVector;

std::hash<std::string> hashFunc;


void load_trace(const std::string &trace_location, trace_vector &trace, int client_batch_size) {
    std::unordered_map<int, std::vector<std::string>> get_keys;
    std::unordered_map<int, std::vector<std::string>> put_keys;
    std::unordered_map<int, std::vector<std::string>> put_values;

    std::unordered_map<std::string, int> key_to_frequency;
    int frequency_sum = 0;
    std::string op, key, val;
    std::ifstream in_workload_file;
    in_workload_file.open(trace_location, std::ios::in);
    if(!in_workload_file){
        std::perror("Unable to find workload file");
    }
    std::string line;
    while (std::getline(in_workload_file, line)) {
        op = line.substr(0, line.find(" "));
        key = line.substr(line.find(" ")+1);
        val = "";
        if (key.find(" ") != -1) {
            val = key.substr(key.find(" ")+1);
            key = key.substr(0, key.find(" "));
        }

        int keyHash = hashFunc(key) % numExecutors;
        if(val == ""){
            get_keys[keyHash].push_back(key);
            if (get_keys[keyHash].size() == client_batch_size){
                trace[keyHash].push_back(std::make_pair(get_keys[keyHash],std::vector<std::string>()));
                get_keys[keyHash].clear();
            }
        }
        else {
            put_keys[keyHash].push_back(key);
            put_values[keyHash].push_back(val);
            if (put_keys[keyHash].size() == client_batch_size){
                trace[keyHash].push_back(std::make_pair(put_keys[keyHash], put_values[keyHash]));
                put_keys[keyHash].clear();
                put_values[keyHash].clear();
            }
        }
        assert (key != "PUT");
        assert (key != "GET");
    }
    for(int i=0; i<numExecutors;i++){

        if (get_keys[i].size() > 0){
            trace[i].push_back(std::make_pair(get_keys[i], std::vector<std::string>()));
            get_keys[i].clear();
        }
        if (put_keys[i].size() > 0){
            trace[i].push_back(std::make_pair(put_keys[i], put_values[i]));
            put_keys[i].clear();
            put_values[i].clear();
        }
    }
    in_workload_file.close();
};


void getKeysValues(const std::string &trace_location, std::vector<std::string>& keys, std::vector<std::string>& values) {
    std::ifstream in_workload_file;
    in_workload_file.open(trace_location, std::ios::in);
    if(!in_workload_file.is_open()){
        std::perror("Unable to find workload file");
    }
    if(in_workload_file.fail()){
        std::perror("Opening workload file failed");
    }
    std::string line;
    std::string op, key, val;
    while (std::getline(in_workload_file, line)) {
        op = line.substr(0, line.find(" "));
        key = line.substr(line.find(" ")+1);
        val = "";

        if (key.find(" ") != -1) {
            val = key.substr(key.find(" ")+1);
            key = key.substr(0, key.find(" "));
        }

        keys.push_back(key);
        values.push_back(val);
        assert (key != "SET");
    }
    in_workload_file.close();
};

void initExecutors(int numExecutor, clientVector clients,std::vector<std::string> keys, std::vector<std::string> values){
    std::unordered_map<int, std::vector<std::string>> execKeys;
    std::unordered_map<int, std::vector<std::string>> execValues;

    for(int i=0; i<keys.size();i++){
        int hashL = hashFunc(keys[i]) % numExecutor;
        execKeys[hashL].push_back(keys[i]);
        execValues[hashL].push_back(values[i]);
    }

    for(int i=0; i<numExecutor; i++){
        std::vector<std::string> thisExecKeys = execKeys[i];
        std::vector<std::string> thisExecValues = execValues[i];
        clients[i].initDb(thisExecKeys, thisExecValues);
    }
    return;
}
void run_benchmark(int run_time, bool stats, std::vector<int> &latencies, int client_batch_size,
                   int object_size, trace_vector &trace, std::atomic<int> &xput, async_proxy_client& client, int client_id) {
    int ops = 0;
    if (stats) {
        ops = client.num_requests_satisfied();
    }
    uint64_t start, end;
    std::vector<std::pair<std::vector<std::string>, std::vector<std::string>>> currTrace = trace[client_id];

    auto ticks_per_ns = static_cast<double>(rdtscuhz()) / 1000;
    auto s = std::chrono::high_resolution_clock::now();
    auto e = std::chrono::high_resolution_clock::now();
    int elapsed = 0;
    int i = 0;
while (elapsed < run_time*1000000) {
        if(i == currTrace.size()) break;
        if (stats) {
            rdtscll(start);
        }
        auto keys_values_pair = currTrace[i];
        if (keys_values_pair.second.empty()){
            client.get_batch(keys_values_pair.first);   
        }
        else {
            
            client.put_batch(keys_values_pair.first, keys_values_pair.second);
        }
        if (stats) {
            rdtscll(end);
            double cycles = static_cast<double>(end - start);
            latencies.push_back((cycles / ticks_per_ns) / client_batch_size);
            rdtscll(start);
            //ops += keys_values_pair.first.size();
        }
        e = std::chrono::high_resolution_clock::now();
        elapsed = static_cast<int>(std::chrono::duration_cast<std::chrono::microseconds>(e - s).count());
        ++i;
    }
    if (stats) 
        ops = client.num_requests_satisfied()- ops;
    // std::cout << "Ops is " << ops << " client num_requests_satisfied is " << client.num_requests_satisfied() << std::endl;
    e = std::chrono::high_resolution_clock::now(); 
    elapsed = static_cast<int>(std::chrono::duration_cast<std::chrono::microseconds>(e - s).count());
    if (stats)
        xput += (int)(static_cast<double>(ops) * 1000000 / elapsed);
}


void clientFunc(int idx,clientVector& clients, int client_batch_size, int object_size, trace_vector &trace, std::atomic<int> &xput) {
    
    std::atomic<int> indiv_xput;
    std::atomic_init(&indiv_xput, 0);
    std::vector<int> latencies;

    std::cout << "Beginning benchmark" << std::endl;
    run_benchmark(30, true, latencies, client_batch_size, object_size, trace, indiv_xput, clients[idx],idx);

    int temp = xput.load();
    xput.store(temp+indiv_xput);

    std::cout << "Benchmark: "<<idx<<" "<<"xput is: " << indiv_xput << std::endl;
    clients[idx].finish();
}





int main(){
    // int  = 1;
    int client_batch_size=800;
    int object_size = 1000;
    std::string output_directory = "data/";
    
    std::string traceLocation = "./tracefiles/0.99/workloada/proxy_server_command_line_input.txt";
    std::string benchTrace = "./tracefiles/0.99/workloada/proxy_benchmark_command_line_input.txt";


    std::vector<std::string> keys;
    std::vector<std::string> values;
    getKeysValues(traceLocation, keys, values);
    clientVector clients;

    async_proxy_client client;
    client.init("127.0.0.1", 9090);
    clients[0]=(client);

    async_proxy_client clientTwo;
    clientTwo.init("127.0.0.1", 9091);
    clients[1]=(clientTwo);

    initExecutors(numExecutors, clients, keys, values);

    trace_vector trace;
    load_trace(benchTrace, trace, client_batch_size);

    std::atomic<int> xput;
    std::atomic_init(&xput, 0);
    std::vector<std::thread> threads;
    for (int i = 0; i < numExecutors; i++) {
        threads.push_back(std::thread(clientFunc, i, std::ref(clients), std::ref(client_batch_size), std::ref(object_size), std::ref(trace), std::ref(xput)));
    }
    for (int i = 0; i < numExecutors; i++)
        threads[i].join();
    sleep(1);
    std::cout << "Xput was: " << xput.load() << std::endl;
    // std::cout << xput << std::endl;
    
}   