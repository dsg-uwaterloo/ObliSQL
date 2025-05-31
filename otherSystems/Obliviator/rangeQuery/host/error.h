#ifndef DISTRIBUTED_SGX_SORT_HOST_ERROR_H
#define DISTRIBUTED_SGX_SORT_HOST_ERROR_H

#define handle_mpi_error(ret, msg) \
    _handle_mpi_error(ret, msg, __FILE__, __LINE__)
void _handle_mpi_error(int ret, const char *msg, const char *file, int line);

#endif /* distributed-sgx-sort/host/error.h */
