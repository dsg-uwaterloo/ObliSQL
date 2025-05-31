#include <stdio.h>
#include <liboblivious/primitives.h>
#include "common/defs.h"
#include "common/elem_t.h"
#include "common/error.h"
#include "common/algorithm_type.h"
#include "common/util.h"
#include "enclave/crypto.h"
#include "enclave/mpi_tls.h"
#include "enclave/threading.h"
#include "enclave/scalable_oblivious_join.h"

#ifndef DISTRIBUTED_SGX_SORT_HOSTONLY
#include <openenclave/enclave.h>
#include "enclave/parallel_t.h"
#endif

int world_rank;
int world_size;

static elem_t *arr;

void ecall_release_threads(void) {
    thread_release_all();
}

void ecall_unrelease_threads(void) {
    thread_unrelease_all();
}

int ecall_ojoin_init(int world_rank_, int world_size_, size_t num_threads) {
    int ret;

    world_rank = world_rank_;
    world_size = world_size_;
    total_num_threads = num_threads;

    ret = rand_init();
    if (ret) {
        handle_error_string("Error initializing RNG");
        goto exit;
    }

    /*
    ret = mpi_tls_init(world_rank, world_size, &entropy_ctx);
    if (ret) {
        handle_error_string("Error initializing MPI-over-TLS");
        goto exit_free_rand;
    }
    */

    scalable_oblivious_join_init(num_threads);

exit:
    return ret;

// exit_free_rand:
    rand_free();
    return ret;
}

void ecall_ojoin_free_arr(void) {
    // mpi_tls_bytes_sent = 0;
}

void ecall_ojoin_free(void) {
    // mpi_tls_free();
    rand_free();
}

void ecall_start_work(void) {
    thread_start_work();
}

int ecall_scalable_oblivious_join(char *input_path, size_t len) {
    (void)len;

    char *length;
    length = strtok(input_path, "\n");
    int length1 = atoi(length);
    int length2 = 0;
    arr = calloc((length1 + length2), sizeof(*arr));
    for (int i = 0; i < length1; i++) {
        arr[i].key = atoi(strtok(NULL, " "));
        strncpy(arr[i].data, strtok(NULL, "\n"), DATA_LENGTH);
    }

    scalable_oblivious_join(arr, length1, length2, input_path);
    
    free(arr);

    return 0;
}