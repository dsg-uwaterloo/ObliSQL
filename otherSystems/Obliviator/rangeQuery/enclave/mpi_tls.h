#ifndef DISTRIBUTED_SGX_SORT_ENCLAVE_MPI_TLS_H
#define DISTRIBUTED_SGX_SORT_ENCLAVE_MPI_TLS_H

#include <stddef.h>
#include <mbedtls/entropy.h>
#include "common/ocalls.h"

enum mpi_tls_request_type {
    MPI_TLS_NULL,
    MPI_TLS_SEND,
    MPI_TLS_RECV,
};

typedef struct mpi_tls_request {
    enum mpi_tls_request_type type;
    ocall_mpi_request_t mpi_request;

    void *buf;
    size_t count;
    struct mpi_tls_msg *msg;
    size_t msg_len;
} mpi_tls_request_t;

typedef ocall_mpi_status_t mpi_tls_status_t;

/* Bandwidth measurement. */
extern size_t mpi_tls_bytes_sent;

#define MPI_TLS_ANY_SOURCE (-2)
#define MPI_TLS_ANY_TAG (-3)
#define MPI_TLS_STATUS_IGNORE ((mpi_tls_status_t *) 0)

int mpi_tls_init(size_t world_rank, size_t world_size,
        mbedtls_entropy_context *entropy);
void mpi_tls_free(void);
int mpi_tls_send_bytes(const void *buf, size_t count, int dest, int tag);
int mpi_tls_recv_bytes(void *buf, size_t count, int src, int tag,
        mpi_tls_status_t *status);
int mpi_tls_isend_bytes(const void *buf, size_t count, int dest, int tag,
        mpi_tls_request_t *request);
int mpi_tls_irecv_bytes(void *buf, size_t count, int src, int tag,
        mpi_tls_request_t *request);
int mpi_tls_wait(mpi_tls_request_t *request, mpi_tls_status_t *status);
int mpi_tls_waitany(size_t count, mpi_tls_request_t *requests, size_t *index,
        mpi_tls_status_t *status);

/* Central location for MPI tags. */

#define BUCKET_DISTRIBUTE_MPI_TAG 1
#define SAMPLE_PARTITION_MPI_TAG 2
#define SAMPLE_PARTITION_DISTRIBUTE_MPI_TAG 3
#define QUICKSELECT_MPI_TAG 4
#define BALANCE_MPI_TAG 5
#define OCOMPACT_MARKED_COUNT_MPI_TAG 6

#endif /* distributed-sgx-sort/enclave/mpi_tls.h */
