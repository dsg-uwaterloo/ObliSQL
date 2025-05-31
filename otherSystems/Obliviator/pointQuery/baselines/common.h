#ifndef DISTRIBUTED_SGX_SORT_BASELINES_COMMON_H
#define DISTRIBUTED_SGX_SORT_BASELINES_COMMON_H

#include <mpi.h>
#include <stdbool.h>
#include <stdio.h>
#include "common/elem_t.h"
#include "host/error.h"

static inline int init_mpi(int *argc, char ***argv, int *world_rank, int *world_size) {
    int ret;

    /* Initialize MPI. */
    int threading_provided;
    ret = MPI_Init_thread(argc, argv, MPI_THREAD_MULTIPLE,
            &threading_provided);
    if (ret) {
        handle_mpi_error(ret, "Error in MPI_Init_thread");
        goto exit;
    }
    if (threading_provided != MPI_THREAD_MULTIPLE) {
        fprintf(stderr, "This program requires MPI_THREAD_MULTIPLE to be supported");
        ret = 1;
        goto exit;
    }

    /* Get world rank and size. */
    ret = MPI_Comm_rank(MPI_COMM_WORLD, world_rank);
    if (ret) {
        handle_mpi_error(ret, "MPI_Comm_rank");
        goto exit;
    }
    ret = MPI_Comm_size(MPI_COMM_WORLD, world_size);
    if (ret) {
        handle_mpi_error(ret, "MPI_Comm_size");
        goto exit;
    }

exit:
    return ret;
}

static inline bool is_pow2(size_t val) {
    size_t i = 1;
    while (i < val) {
        i <<= 1;
    }
    return i == val;
}

static inline int elem_comparator(const void *a_, const void *b_) {
    const elem_t *a = a_;
    const elem_t *b = b_;
    return (a->key > b->key) - (a->key < b->key);
}

#endif /* distributed-sgx-sort/baselines/common.h */
