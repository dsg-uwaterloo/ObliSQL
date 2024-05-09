#ifndef WAFFLE_PROXY_CLIENT_H
#define WAFFLE_PROXY_CLIENT_H

#include <atomic>
#include <thrift/transport/TSocket.h>
#include <thrift/protocol/TBinaryProtocol.h>
#include <thrift/transport/TBufferTransports.h>
#include <thrift/transport/TTransportUtils.h>
#include <string>
#include <vector>
#include <atomic>
#include "client.h"
#include "waffle_thrift.h"

using namespace apache::thrift;
using namespace apache::thrift::protocol;
using namespace apache::thrift::transport;

class proxy_client : client{

public:
    void init(const std::string &host_name, int port) override;
    std::string get(const std::string &key) override;
    void put(const std::string &key, const std::string &value) override;
    std::vector<std::string> get_batch(const std::vector<std::string> &keys) override;
    void put_batch(const std::vector<std::string> &keys, const std::vector<std::string> &values) override;
    int num_requests_satisfied();
    void reset_requests();
    void finish();
    void initDb(const std::vector<std::string> &keys, const std::vector<std::string> &values);
private:
    std::shared_ptr<waffle_thriftClient> client_;
    std::atomic_int* total_;

     /* Transport */
    std::shared_ptr<apache::thrift::transport::TTransport> transport{};
    /* Protocol */
    std::shared_ptr<apache::thrift::protocol::TProtocol> protocol{};
};


#endif //waffle_PROXY_CLIENT_H
