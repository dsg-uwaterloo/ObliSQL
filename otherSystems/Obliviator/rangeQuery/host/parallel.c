#include <errno.h>
#include <pthread.h>
#include <stddef.h>
#include <stdio.h>
#include <stdlib.h>
#include <time.h>
#include <mpi.h>
#include "common/error.h"
#include "common/ocalls.h"
#include "common/algorithm_type.h"
#include "host/error.h"

#ifndef DISTRIBUTED_SGX_SORT_HOSTONLY
#include <openenclave/host.h>
#include "host/parallel_u.h"
#endif /* DISTRUBTED_SGX_SORT_HOSTONLY */

#define MAX_BUF_SIZE 1073741824

static int world_rank;
static int world_size;

FILE *input_file;
FILE *output_file;

/*
static int init_mpi(int *argc, char ***argv) {
    int ret;

    int threading_provided;
    ret = MPI_Init_thread(argc, argv, MPI_THREAD_MULTIPLE, &threading_provided);
    if (ret) {
        handle_mpi_error(ret, "MPI_Init_thread");
        goto exit;
    }
    if (threading_provided != MPI_THREAD_MULTIPLE) {
        printf("This program requires MPI_THREAD_MULTIPLE to be supported");
        ret = 1;
        goto exit;
    }

    ret = MPI_Comm_rank(MPI_COMM_WORLD, &world_rank);
    if (ret) {
        handle_mpi_error(ret, "MPI_Comm_rank");
        goto exit;
    }
    ret = MPI_Comm_size(MPI_COMM_WORLD, &world_size);
    if (ret) {
        handle_mpi_error(ret, "MPI_Comm_size");
        goto exit;
    }

exit:
    return ret;
}
*/

static void *start_thread_work(void *enclave_) {
#ifndef DISTRIBUTED_SGX_SORT_HOSTONLY
    oe_enclave_t *enclave = enclave_;
    oe_result_t result = ecall_start_work(enclave);
    if (result != OE_OK) {
        handle_oe_error(result, "ecall_start_work");
    }
#else /* DISTRIBUTED_SGX_SORT_HOSTONLY */
    ecall_start_work();
#endif /* DISTRIBUTED_SGX_SORT_HOSTONLY */
    return 0;
}

#ifndef DISTRIBUTED_SGX_SORT_HOSTONLY

int time_join(oe_enclave_t *enclave) {
    oe_result_t result;
#else

int time_join(enum algorithm_type algorithm_type) {
#endif
    int ret;


    struct timespec start;
    ret = timespec_get(&start, TIME_UTC);
    if (!ret) {
        perror("starting timespec_get");
        goto exit_free_arr;
    }

char *buf = (char *)malloc(MAX_BUF_SIZE);
ret = fread(buf, 1, MAX_BUF_SIZE, input_file);
fclose(input_file);


#ifndef DISTRIBUTED_SGX_SORT_HOSTONLY

    result = ecall_scalable_oblivious_join(enclave, &ret, buf, MAX_BUF_SIZE);
   
    if (result != OE_OK) {
        goto exit_free_arr;
    }
#else /* DISTRIBUTED_SGX_SORT_HOSTONLY */
            ret = ecall_scalable_oblivious_join(buf, MAX_BUF_SIZE);
#endif /* DISTRIBUTED_SGX_SORT_HOSTONLY */
    if (ret) {
        handle_error_string("Enclave exited with return code %d", ret);
        goto exit_free_arr;
    }

    fwrite(buf, 1, strlen(buf), output_file);
    fclose(output_file);
    free(buf);
    // MPI_Barrier(MPI_COMM_WORLD);

    struct timespec end;
    ret = timespec_get(&end, TIME_UTC);
    if (!ret) {
        perror("ending timespec_get");
        goto exit_free_arr;
    }

exit_free_arr:
#ifndef DISTRIBUTED_SGX_SORT_HOSTONLY
    result = ecall_ojoin_free_arr(enclave);
    if (result != OE_OK) {
        handle_oe_error(result, "ecall_ojoin_free_arr");
    }
#else
    ecall_ojoin_free_arr();
#endif

    return ret;
}

int main(int argc, char **argv) {
    int ret = -1;

    /* Read arguments. */

#ifndef DISTRIBUTED_SGX_SORT_HOSTONLY
    if (argc < 4) {
        printf("usage: %s enclave_image num_threads input_file_path\n", argv[0]);
#else /* DISTRIBUTED_SGX_SORT_HOSTONLY */
    if (argc < 3) {
        printf("usage: %s num_threads input_file_path\n", argv[0]);
#endif /* DISTRIBUTED_SGX_SORT_HOSTONLY */
        return 0;
    }
    

    errno = 0;
#ifndef DISTRIBUTED_SGX_SORT_HOSTONLY
    size_t num_threads = strtoll(argv[2], NULL, 10);
#else /* DISTRIBUTED_SGX_SORT_HOSTONLY */
    size_t num_threads = strtoll(argv[2], NULL, 10);
#endif /* DISTRIBUTED_SGX_SORT_HOSTONLY */
    if (errno) {
        printf("Invalid number of threads\n");
        return ret;
    }

    //size_t num_runs = 1;

    /* Init MPI. */

    // ret = init_mpi(&argc, &argv);
    pthread_t threads[num_threads - 1];

    /* Create enclave. */
    /*
    if (ret) {
        handle_error_string("init_mpi");
        goto exit_mpi_finalize;
    }
    */

#ifndef DISTRIBUTED_SGX_SORT_HOSTONLY
    oe_enclave_t *enclave;
    oe_result_t result;
    result = oe_create_parallel_enclave(
            argv[1],
            OE_ENCLAVE_TYPE_AUTO,
            0,
            NULL,
            0,
            &enclave);

    if (result != OE_OK) {
        handle_oe_error(result, "oe_create_parallel_enclave");
        ret = result;
        goto exit_mpi_finalize;
    }
#endif /* DISTRIBUTED_SGX_SORT_HOSTONLY */

    /* Init enclave with threads. */

#ifndef DISTRIBUTED_SGX_SORT_HOSTONLY
    result =
            ecall_ojoin_init(enclave, &ret, world_rank, world_size, num_threads);
    if (result != OE_OK) {
        handle_oe_error(result, "ecall_ojoin_init");
        goto exit_terminate_enclave;
    }
#else /* DISTRIBUTED_SGX_SORT_HOSTONLY */
    ret = ecall_ojoin_init(world_rank, world_size, num_threads);
#endif /* DISTRIBUTED_SGX_SORT_HOSTONLY */
    if (ret) {
        handle_error_string("Error in enclave joining initialization");
        goto exit_terminate_enclave;
    }

    for (size_t i = 1; i < num_threads; i++) {
#ifndef DISTRIBUTED_SGX_SORT_HOSTONLY
        ret = pthread_create(&threads[i - 1], NULL, start_thread_work, enclave);
#else /* DISTRIBUTED_SGX_SORT_HOSTONLY */
        ret = pthread_create(&threads[i - 1], NULL, start_thread_work, NULL);
#endif /* DISTRIBUTED_SGX_SORT_HOSTONLY */
        if (ret) {
            perror("pthread_create");
            goto exit_free_sort;
        }
    }

    for (int i = 3; i < argc; i++) {
        input_file = fopen(argv[i], "rb");
        char *output_file_path = calloc(strlen(argv[i]) + 8, sizeof(*output_file_path));
        for (size_t u = 0; u < strlen(argv[i]) - 4; u++) {
            output_file_path[u] = argv[i][u];
        }
        output_file_path[strlen(argv[i]) - 4] = '_';
        output_file_path[strlen(argv[i]) - 3] = 'o';
        output_file_path[strlen(argv[i]) - 2] = 'u';
        output_file_path[strlen(argv[i]) - 1] = 't';
        output_file_path[strlen(argv[i])] = 'p';
        output_file_path[strlen(argv[i]) + 1] = 'u';
        output_file_path[strlen(argv[i]) + 2] = 't';
        output_file_path[strlen(argv[i]) + 3] = '.';
        output_file_path[strlen(argv[i]) + 4] = 't';
        output_file_path[strlen(argv[i]) + 5] = 'x';
        output_file_path[strlen(argv[i]) + 6] = 't';
        output_file_path[strlen(argv[i]) + 7] = '\0';
        output_file = fopen(output_file_path, "w");
        if (!output_file) {
            printf("Invalid OUTPUT FILE\n");
            return 0;
        }
#ifndef DISTRIBUTED_SGX_SORT_HOSTONLY
        ret = time_join(enclave);
#else
        ret = time_join(algorithm_type);
#endif

    }

#ifndef DISTRIBUTED_SGX_SORT_HOSTONLY
    result = ecall_release_threads(enclave);
    if (result != OE_OK) {
        handle_oe_error(result, "ecall_release_threads");
    }
#else /* DISTRIBUTED_SGX_SORT_HOSTONLY */
    ecall_release_threads();
#endif
    for (size_t i = 1; i < num_threads; i++) {
        pthread_join(threads[i - 1], NULL);
    }

#ifndef DISTRIBUTED_SGX_SORT_HOSTONLY
    result = ecall_unrelease_threads(enclave);
    if (result != OE_OK) {
        handle_oe_error(result, "ecall_release_threads");
    }
#else /* DISTRIBUTED_SGX_SORT_HOSTONLY */
    ecall_unrelease_threads();
#endif

exit_free_sort:
#ifndef DISTRIBUTED_SGX_SORT_HOSTONLY
    result = ecall_ojoin_free(enclave);
    if (result != OE_OK) {
        handle_oe_error(result, "ecall_sort_free");
    }
#else
    ecall_ojoin_free();
#endif
exit_terminate_enclave:
#ifndef DISTRIBUTED_SGX_SORT_HOSTONLY
    oe_terminate_enclave(enclave);
#endif
exit_mpi_finalize:
    
    return ret;
}
