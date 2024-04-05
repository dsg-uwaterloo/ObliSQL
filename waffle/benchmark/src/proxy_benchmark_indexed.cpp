#include <cstdint>
#include <cstdlib>
#include <map>
#include <ostream>
#include <string>
#include <unordered_map>
#include <fstream>
#include <iostream>
#include <sstream>
#include <sys/stat.h>
#include <thread>
#include <utility>
#include <vector>
#include "timer.h"
#include "waffle_proxy.h"
#include "thrift_server.h"
#include "proxy_client.h"
#include "thrift_utils.h"

typedef std::vector<std::pair<std::vector<std::string>, std::vector<std::string>>> trace_vector;
typedef std::map<std::string,std::vector<std::string>> metaData;

void generateFullTableKeys(trace_vector &trace, metaData m,std::string colName,std::string tableName,long clientBatchSize){

    std::vector<std::string> get_keys;
    std::vector<std::string> colNames = m["colNames"];
    int pkstart = std::stoi(m["pkStart"].front());
    int pkEnd = std::stoi(m["pkEnd"].front());

    if(colName==""){
        while(pkstart!=pkEnd){
            for(auto col: colNames){
                std::string key = tableName + "/" + col + "/" + std::to_string(pkstart);
                get_keys.push_back(key);
                if(get_keys.size() == clientBatchSize){
                    // std::cout<<get_keys[0]<<std::endl;
                    // std::cout<<"Added to Trace Size:"<<get_keys.size()<<std::endl;
                    trace.push_back(std::make_pair(get_keys, std::vector<std::string>()));
                    get_keys.clear();
                }
            }
            pkstart++;
        }
    }else{
        while(pkstart!=pkEnd){
            std::string key = tableName + "/" + colName + "/" + std::to_string(pkstart);
            get_keys.push_back(key);
            if(get_keys.size() == clientBatchSize){
                trace.push_back(std::make_pair(get_keys, std::vector<std::string>()));
            }
            pkstart++;
        }
    }
    if(get_keys.size()>0){
        // std::cout<<"Trace Left:"<<get_keys.size()<<std::endl;
        long dummy = clientBatchSize - get_keys.size();
        for(int i=0; i<dummy;i++){
            get_keys.push_back(get_keys[i]);
        }
        trace.push_back(std::make_pair(get_keys, std::vector<std::string>()));
    }
}

void usage() {
    std::cout << "Proxy client\n";
    std::cout << "\t -h: Proxy host name\n";
    std::cout << "\t -p: Proxy port\n";
    std::cout << "\t -t: Trace Location\n";
    std::cout << "\t -n: Number of threads to spawn\n";
    std::cout << "\t -s: Object Size\n";
    std::cout << "\t -o: Output Directory\n";
};

void readMetaDataFromFile(metaData &m) {
    std::ifstream metaDataFile;
    metaDataFile.open("./benchmark/src/metadata.txt",std::ios::in);
    if (metaDataFile.is_open()) {
        std::string line;
        while (std::getline(metaDataFile, line)) {
            std::istringstream iss(line);
            std::string key;
            std::string value;
            if (std::getline(iss, key, ':') && std::getline(iss, value)) {
                std::istringstream valueStream(value);
                std::string token;
                while (valueStream >> token) {
                    m[key].push_back(token);
                }
            }
        }
        metaDataFile.close();
        std::cout << "MetaData read from metadata.txt" << std::endl;
    }
};

std::string splitStringAndGetFirst(const std::string& input,std::string delim) {
    std::string result;
    std::istringstream iss(input);
    std::getline(iss, result, delim[0]);
    return result;
}

void fetchIndexedTable(bool stats,std::string tableName, std::string fetchColumn, std::string searchColumn, std::string searchValue, int client_batch_size, proxy_client client,std::atomic<int> &xput,std::vector<int> &latencies) {
    long ops = 0;
    if (stats) {
        ops = 0;
    }
    uint64_t start, end;
    //std::cout << "Entering proxy_benchmark.cpp line " << __LINE__ << std::endl;
    auto ticks_per_ns = static_cast<double>(rdtscuhz()) / 1000;
    auto s = std::chrono::high_resolution_clock::now();
    auto e = std::chrono::high_resolution_clock::now();
    int elapsed = 0;

    if (stats) {
        rdtscll(start);
    }

    std::string indexKey = tableName + "/" + searchColumn + "_index" + "/" + searchValue;
    std::vector<std::string> indexKeys;
    for (int i = 0; i < client_batch_size; i++) {
        indexKeys.push_back(indexKey);
    }
    auto indexedKeys = client.get_batch(indexKeys);

    std::vector<std::string> splitKeys;
    std::istringstream iss(indexedKeys[0]);
    std::string token;
    while (std::getline(iss, token, ',')) {
        splitKeys.push_back(splitStringAndGetFirst(token,"|"));
    }
    std::vector<std::string> objKeyVec;
    for (const auto& keyy : splitKeys) {
        std::cout<<"Key: "<<keyy<<std::endl;
       std::string objectKey = tableName+"/"+fetchColumn+"/"+keyy;
       objKeyVec.push_back(objectKey);
    }
    int totalSize = client_batch_size - objKeyVec.size();
    for (int i = 0; i < totalSize; i++) {
        objKeyVec.push_back(objKeyVec[0]);
    }
    auto result = client.get_batch(objKeyVec);
    if (stats) {
        rdtscll(end);
        double cycles = static_cast<double>(end - start);
        latencies.push_back((cycles / ticks_per_ns) / client_batch_size); //Convert Nano seconds to micro seconds. 
        rdtscll(start);
    }
    
    for(int i=0; i<10;i++){
        std::cout<<result[i]<<std::endl;
    }
    e = std::chrono::high_resolution_clock::now(); 
    elapsed = static_cast<int>(std::chrono::duration_cast<std::chrono::microseconds>(e - s).count());
}

void run_benchmark(int run_time, bool stats, std::vector<int> &latencies, int client_batch_size,
                   int object_size, trace_vector &trace, std::atomic<int> &xput, proxy_client client) {
    std::string dummy(object_size, '0');
    int ops = client.num_requests_satisfied();
    if (stats) {
        ops = 0;
    }
    uint64_t start, end;
    //std::cout << "Entering proxy_benchmark.cpp line " << __LINE__ << std::endl;
    auto ticks_per_ns = static_cast<double>(rdtscuhz()) / 1000;
    auto s = std::chrono::high_resolution_clock::now();
    auto e = std::chrono::high_resolution_clock::now();
    int elapsed = 0;
    std::vector<std::string> results;
    int i = 0;
    while (elapsed < run_time*1000000) {
        if(i == trace.size()) break;
    
        auto keys_values_pair = trace[i];
        if (keys_values_pair.second.empty()){
            //std::cout << "Entering proxy_benchmark.cpp line " << __LINE__ << std::endl;
            auto res = client.get_batch(keys_values_pair.first);
        }
        else {
            //std::cout << "Entering proxy_benchmark.cpp line " << __LINE__ << std::endl;
            client.put_batch(keys_values_pair.first, keys_values_pair.second);
        }
        e = std::chrono::high_resolution_clock::now();
        elapsed = static_cast<int>(std::chrono::duration_cast<std::chrono::microseconds>(e - s).count());
        ++i;
    }
}

void warmup(std::vector<int> &latencies, int client_batch_size,
            int object_size, trace_vector &trace, std::atomic<int> &xput, proxy_client client) {
    run_benchmark(5, false, latencies, client_batch_size, object_size, trace, xput, client);
}

void cooldown(std::vector<int> &latencies, int client_batch_size,
              int object_size, trace_vector &trace, std::atomic<int> &xput, proxy_client client) {
    run_benchmark(5, false, latencies, client_batch_size, object_size, trace, xput, client);
}

void client(int idx, int client_batch_size, int object_size, trace_vector &trace, std::string &output_directory, std::string &host, int proxy_port, metaData m,std::atomic<int> &xput) {
    proxy_client client;
    client.init(host, proxy_port);

    std::atomic<int> indiv_xput;
    std::atomic_init(&indiv_xput, 0);
    std::vector<int> latencies;
    std::cout << "Beginning warmup" << std::endl;
    warmup(latencies, client_batch_size, object_size, trace, indiv_xput, client);
        
        //Select c_zip from customer where c_state = "B1";
    std::cout << "Beginning benchmark" << std::endl;
    fetchIndexedTable(true,m["tableName"].front(),"c_zip","c_state","B1",client_batch_size,client,xput,latencies);

    // run_benchmark(30, true, latencies, client_batch_size, object_size, trace, indiv_xput, client);

    std::string location = output_directory + "/" + std::to_string(idx);
    std::ofstream out(location);
    std::string line("");
    for (auto lat : latencies) {
        line.append(std::to_string(lat) + "\n");
        out << line;
        line.clear();
    }
    line.append("Xput: " + std::to_string(indiv_xput) + "\n");
    out << line;
    xput += indiv_xput;

    // std::cout << "xput is: " << xput << std::endl;
    std::cout << "Beginning cooldown" << std::endl;
    cooldown(latencies, client_batch_size, object_size, trace, indiv_xput, client);

    client.finish();
}


int main(int argc, char *argv[]){
    std::string proxy_host = "127.0.0.1";
    int proxy_port = 9090;
    std::string trace_location = "";
    int client_batch_size = 800;
    int object_size = 255;
    int num_clients=1;

    metaData tableMeta={};
    readMetaDataFromFile(tableMeta);

    std::time_t end_time = std::chrono::system_clock::to_time_t(std::chrono::system_clock::now());
    auto date_string = std::string(std::ctime(&end_time));
    date_string = date_string.substr(0, date_string.rfind(":"));
    date_string.erase(remove(date_string.begin(), date_string.end(), ' '), date_string.end());
    date_string=date_string+"Indexed";
    std::string output_directory = "./data/"+date_string;

    int o;
    std::string proxy_type_ = "pancake";
    while ((o = getopt(argc, argv, "h:p:t:s:b:n:o:")) != -1) {
        switch (o) {
            case 'h':
                proxy_host = std::string(optarg);
                break;
            case 'p':
                proxy_port = std::atoi(optarg);
                break;
            case 't':
                trace_location = std::string(optarg);
                break;
            case 's':
                object_size = std::atoi(optarg);
                break;
            case 'n':
                num_clients = std::atoi(optarg);
                break;
            case 'o':
                output_directory = std::string(optarg);
                break;
            default:
                usage();
                exit(-1);
        }
    }

    
    mkdir((output_directory).c_str(), 0777);
    std::atomic<int> xput;
    std::atomic_init(&xput, 0);

    trace_vector trace;
    generateFullTableKeys(trace,tableMeta,"","customer",client_batch_size);
    std::cout<<"Trace Created of Size: "<<trace.size()<<std::endl;

    std::vector<std::thread> threads;
    for (int i = 0; i < num_clients; i++) {
        threads.push_back(std::thread(client, std::ref(i), std::ref(client_batch_size), std::ref(object_size), std::ref(trace),
                          std::ref(output_directory), std::ref(proxy_host), std::ref(proxy_port),std::ref(tableMeta), std::ref(xput)));
    }
    for (int i = 0; i < num_clients; i++)
        threads[i].join();
    // std::cout << "Xput was: " << xput << std::endl;
    std::cout << xput << std::endl;
}