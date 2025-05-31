#ifndef DISTRIBUTED_SGX_SORT_COMMON_OCALLS_H
#define DISTRIBUTED_SGX_SORT_COMMON_OCALLS_H

#include <stddef.h>

#define OCALL_MPI_ANY_SOURCE (-2)
#define OCALL_MPI_ANY_TAG (-3)

typedef struct ocall_mpi_status {
    int count;
    int source;
    int tag;
} ocall_mpi_status_t;

typedef struct ocall_mpi_request * ocall_mpi_request_t;

struct ocall_enclave_stats {
    size_t mpi_tls_bytes_sent;
};

#define OCALL_MPI_REQUEST_NULL ((ocall_mpi_request_t) 0)

#endif /* distributed-sgx-sort/common/ocalls.h */
