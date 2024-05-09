#include "proxy_client.h"
#include <iostream>

using namespace ::apache::thrift;
using namespace ::apache::thrift::protocol;
using namespace ::apache::thrift::transport;

void proxy_client::init(const std::string &host_name, int port) {
    auto socket = std::make_shared<TSocket>(host_name, port);
    socket->setRecvTimeout(10000);
    socket->setSendTimeout(1200000);
    transport = std::shared_ptr<TTransport>(new TFramedTransport(socket));
    protocol = std::shared_ptr<TProtocol>(new TBinaryProtocol(transport));
    client_ = std::make_shared<waffle_thriftClient>(protocol);
    total_ = new std::atomic<int>(0);
    transport->open();
}

std::string proxy_client::get(const std::string &key) {
    std::string _return;
    client_->get(_return, key);
    *total_ +=_return.size();
    return _return;
}

void proxy_client::put(const std::string &key, const std::string &value) {
    //std::string _return;
    client_->put(key, value);
}

std::vector<std::string> proxy_client::get_batch(const std::vector<std::string> &keys) {
    // std::cout<<"Get Batch Called"<<std::endl;
    std::vector<std::string> _return;
    client_->get_batch(_return, keys);
    *total_ +=_return.size();
    return _return;
}

void proxy_client::put_batch(const std::vector<std::string> &keys, const std::vector<std::string> &values) {
    std::string _return;
    client_->put_batch(keys, values);
}

int proxy_client::num_requests_satisfied(){
    return total_->load();
}
void proxy_client::reset_requests(){
    total_->store(0);
}

void proxy_client::finish(){
    transport->close();
}

void proxy_client::initDb(const std::vector<std::string> &keys, const std::vector<std::string> &values){
    client_->init_db(keys, values);
}