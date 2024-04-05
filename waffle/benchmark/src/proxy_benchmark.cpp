#include <functional>
#include <queue>
#include <unistd.h>
#include <unordered_map>
#include <fstream>
#include <iostream>
#include <sstream>
#include <sys/stat.h>
#include <thread>
#include <vector>
#include "client.h"
#include "timer.h"
#include "waffle_proxy.h"
#include "thrift_server.h"
#include "proxy_client.h"
#include "thrift_utils.h"

typedef std::vector<std::pair<std::vector<std::string>, std::vector<std::string>>> trace_vector;
std::vector<proxy_client*> clients;
std::vector<std::queue<std::string>> executors; 
std::vector<std::pair<std::thread, std::shared_ptr<waffle_proxy>>> proxyServers;

void load_trace(const std::string &trace_location, trace_vector &trace, int client_batch_size) {
    std::vector<std::string> get_keys;
    std::vector<std::string> put_keys;
    std::vector<std::string> put_values;

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
        if(val == ""){
            get_keys.push_back(key);
            if (get_keys.size() == client_batch_size){
                trace.push_back(std::make_pair(get_keys, std::vector<std::string>()));
                get_keys.clear();
            }
        }
        else {
            put_keys.push_back(key);
            put_values.push_back(val);
            if (put_keys.size() == client_batch_size){
                trace.push_back(std::make_pair(put_keys, put_values));
                put_keys.clear();
                put_values.clear();
            }
        }
        assert (key != "PUT");
        assert (key != "GET");
    }
    if (get_keys.size() > 0){
        trace.push_back(std::make_pair(get_keys, std::vector<std::string>()));
        get_keys.clear();
    }
    if (put_keys.size() > 0){
        trace.push_back(std::make_pair(put_keys, put_values));
        put_keys.clear();
        put_values.clear();
    }
    in_workload_file.close();
};

void launchProxyServer(std::string proxyHost, int proxyPort, std::vector<std::string> putKeys, std::vector<std::string> putValues,int clientId){
    
    void *arguments[1];
    int B = 1200;
    int R = 800;
    int F = 100;
    int D = 100000;
    int c = 2;
    int num_cores = 2;
    std::shared_ptr<waffle_proxy> proxy_ = std::make_shared<waffle_proxy>();
    dynamic_cast<waffle_proxy&>(*proxy_).server_host_name_ = "192.168.152.192";
    // dynamic_cast<waffle_proxy&>(*proxy_).server_host_name_ = proxyHost;
    dynamic_cast<waffle_proxy&>(*proxy_).server_port_ = 6379;
    dynamic_cast<waffle_proxy&>(*proxy_).B = B;
    dynamic_cast<waffle_proxy&>(*proxy_).R = R;
    dynamic_cast<waffle_proxy&>(*proxy_).F = F;
    dynamic_cast<waffle_proxy&>(*proxy_).D = D;    
    dynamic_cast<waffle_proxy&>(*proxy_).cacheBatches = c;
    dynamic_cast<waffle_proxy&>(*proxy_).num_cores = num_cores; 
    

    auto id_to_client = std::make_shared<thrift_response_client_map>();
    arguments[0] = &id_to_client;
    std::cout <<"Initializing Waffle" << std::endl;
    dynamic_cast<waffle_proxy&>(*proxy_).init(putKeys, putValues, arguments);
    std::cout << "Initialized Waffle" << std::endl;
    auto proxy_server = thrift_server::create(proxy_, "waffle", id_to_client, proxyPort, 1);
    std::thread serverThread ([proxy_server] { proxy_server->serve(); });
    proxyServers.emplace_back(std::move(serverThread),proxy_);
    std::cout << "Proxy server is reachable" <<proxyHost<<":"<<proxyPort<<std::endl;
    
}

void hashInput(int numExecutors,trace_vector inputVector,std::string proxyHost, int proxyPort){

    std::vector<std::queue<std::pair<std::string, std::string>>> assignment(numExecutors); 
    for(auto &pair: inputVector){
        for(int i=0; i<pair.first.size();i++){
            size_t hash = std::hash<std::string>{}(pair.first[i]) % numExecutors;
            assignment[hash].push((std::make_pair(pair.first[i], pair.second[i])));
        }
    }
    for(int i =0; i<assignment.size();i++){
        std::vector<std::string> put_keys;
        std::vector<std::string> put_values;
        std::queue<std::pair<std::string, std::string>> currAssignment = assignment[i];
        while (!currAssignment.empty()) {
            std::pair<std::string, std::string> kvPair = currAssignment.front();
            put_keys.push_back(kvPair.first);
            put_values.push_back(kvPair.second);
            currAssignment.pop();
        }
        launchProxyServer(proxyHost,proxyPort+i,put_keys,put_values,i);
    }
}

void initClients(int numExecutors,std::string proxyHost, int proxyPort){
    for(int i=0;i<numExecutors;i++){
        proxy_client* client = new proxy_client;
        client->init(proxyHost, proxyPort+i);
        clients.push_back(client);
    }
    std::cout<<"Clients Initialized!"<<std::endl;
}

void syncGet(int clientId, int client_batch_size){
    std::vector<std::string> getkeys;
    while(getkeys.size() <client_batch_size){
        getkeys.push_back(executors[clientId].front());
        executors[clientId].pop();
    }
    std::vector<std::string> ret = clients[clientId]->get_batch(getkeys);
}

void doBenchmark(int run_time, bool stats, std::vector<int> &latencies, int client_batch_size, int object_size, trace_vector &trace, std::atomic<int> &xput){
    int ops = 0;
    std::vector<std::thread> batchThreads;
    uint64_t start, end;
    // std::cout << "Running Benchmark! " << __LINE__ << std::endl;
    auto ticks_per_ns = static_cast<double>(rdtscuhz()) / 1000;
    auto s = std::chrono::high_resolution_clock::now();
    auto e = std::chrono::high_resolution_clock::now();
    int elapsed = 0;
    int i=0;
    while (elapsed < run_time * 1000000) {
        // std::cout<<"Elapsed:"<<elapsed<<std::endl;
        if(i==trace.size()){
        //   std::cout<<"Trace Finished"<<std::endl;
          break;  
        } 
        if (stats) {
            rdtscll(start);
        }
        auto keys_values_pair = trace[i];
        if(keys_values_pair.second.empty()){
            for(int i=0; i<keys_values_pair.first.size();i++){ //Only Get Requests;
                size_t hash = std::hash<std::string>{}(keys_values_pair.first[i]) % clients.size();
                executors[hash].push(keys_values_pair.first[i]);
                bool allFull = std::all_of(executors.begin(), executors.end(),
                        [client_batch_size](const std::queue<std::string>& q) { return q.size() >= client_batch_size; });
                
                if(allFull){
                    std::vector<std::thread> thisBatch;
                    for(size_t clientId = 0; clientId<clients.size();++clientId){
                        thisBatch.push_back(std::thread([clientId,client_batch_size]() {syncGet(clientId, client_batch_size);}));

                    }
                    for(int i=0; i<clients.size();i++){
                        thisBatch[i].join();
                    }
                    if (stats) {
                        rdtscll(end);
                        double cycles = static_cast<double>(end - start);
                        latencies.push_back((cycles / ticks_per_ns) / client_batch_size);
                        rdtscll(start);
                        //ops += keys_values_pair.first.size();
                    }
                }
            }
        }
        e = std::chrono::high_resolution_clock::now();
        elapsed = static_cast<int>(std::chrono::duration_cast<std::chrono::microseconds>(e - s).count());
        ++i;
    }

    if (stats) 
        for(auto& client:clients){
        int numOps = client->num_requests_satisfied();
        // std::cout<<"NumOps: "<<numOps<<std::endl;
        ops+=numOps;
    }else if(stats == false){
        for(auto& client:clients){
            client->reset_requests();
        }
    }
    e = std::chrono::high_resolution_clock::now(); 
    elapsed = static_cast<int>(std::chrono::duration_cast<std::chrono::microseconds>(e - s).count());
    if (stats)
        xput += (int)(static_cast<double>(ops) * 1000000 / elapsed);
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

int _mkdir(const char *path) {
    #ifdef _WIN32
        return ::_mkdir(path);
    #else
        #if _POSIX_C_SOURCE
            return ::mkdir(path, 0755);
        #else
            return ::mkdir(path, 0755); // not sure if this works on mac
        #endif
    #endif
}

int main(int argc, char *argv[]) {
    std::string proxy_host = "127.0.0.1";
    int proxy_port = 9090;
    int client_batch_size = 800;
    int object_size = 1000;
    int executorsNum = 1;
    int num_clients = 1;

    int o;
    while ((o = getopt(argc, argv, "e:")) != -1) {
        switch (o) {
            case 'e':
                executorsNum = std::atoi(optarg);
                break;
            default:
                exit(-1);
        }
    }
    std::cout<<"Executors: "<<executorsNum<<std::endl;

    executors.resize(executorsNum);
    std::time_t end_time = std::chrono::system_clock::to_time_t(std::chrono::system_clock::now());
    auto date_string = std::string(std::ctime(&end_time));
    date_string = date_string.substr(0, date_string.rfind(":"));
    date_string.erase(remove(date_string.begin(), date_string.end(), ' '), date_string.end());
    std::string output_directory = "./data/"+date_string;


    _mkdir((output_directory).c_str());
    std::atomic<int> xput;
    std::atomic_init(&xput, 0);
    std::vector<int> latencies;

    std::string inputTraceLocation = "./tracefiles/0.99/workloada/proxy_server_command_line_input.txt";
    std::string benchTraceLocation = "./tracefiles/0.99/workloada/proxy_benchmark_command_line_input.txt";

    trace_vector inputTrace;
    load_trace(inputTraceLocation, inputTrace, client_batch_size);

    trace_vector benchtrace;
    load_trace(benchTraceLocation, benchtrace, client_batch_size);
    

    hashInput(executorsNum,inputTrace,proxy_host,proxy_port);
    sleep(1);
    initClients(executorsNum,proxy_host,proxy_port);
   
    doBenchmark(15,false,latencies,client_batch_size,object_size,benchtrace,xput);

    doBenchmark(30,true,latencies,client_batch_size,object_size,benchtrace,xput);

    std::string location = output_directory + "/" + std::to_string(1);
    std::ofstream out(location);
    std::string line("");
    for (auto lat : latencies) {
        line.append(std::to_string(lat) + "\n");
        out << line;
        line.clear();
    }

//  std::cout<<"Doing Cooldown"<<std::endl;
//     doBenchmark(15,false,latencies,client_batch_size,object_size,benchtrace,xput);

// std::cout<<"Finished!"<<std::endl;
std::cout<<"XPUT:" << xput << std::endl;

}
