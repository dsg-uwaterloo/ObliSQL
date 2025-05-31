#include <errno.h>
#include <liboblivious/primitives.h>
#include <mpi.h>
#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include "baselines/common.h"
#include "common/elem_t.h"
#include "common/error.h"
#include "host/error.h"

#define SWAP_CHUNK_SIZE 4096

#define MIN(a, b) ((a) < (b) ? (a) : (b))
#define MAX(a, b) ((a) > (b) ? (a) : (b))

static int world_rank;
static int world_size;

static size_t total_length;
static size_t local_length;

static _Thread_local elem_t *buffer;

/* Array index and world rank relationship helpers. */

static int get_index_address(size_t index) {
    return index * world_size / total_length;
}

static size_t get_local_start(int rank) {
    return (rank * total_length + world_size - 1) / world_size;
}

/* Swapping. */

static void swap_local_range(elem_t *arr, size_t a, size_t b, size_t count,
        bool descending) {
    size_t local_start = get_local_start(world_rank);
    for (size_t i = 0; i < count; i++) {
        size_t a_local_idx = a + i - local_start;
        size_t b_local_idx = b + i - local_start;
        bool cond =
            (arr[a_local_idx].key > arr[b_local_idx ].key) != descending;
        o_memswap(&arr[a_local_idx], &arr[b_local_idx], sizeof(*arr), cond);
    }
}

static void swap_remote_range(elem_t *arr, size_t local_idx, size_t remote_idx,
        size_t count, bool descending) {
    size_t local_start = get_local_start(world_rank);
    int remote_rank = get_index_address(remote_idx);
    int ret;

    /* Swap elems in maximum chunk sizes of SWAP_CHUNK_SIZE and iterate until
     * no count is remaining. */
    while (count) {
        size_t elems_to_swap = MIN(count, SWAP_CHUNK_SIZE);

        /* Post receive for remote elems to encrypted buffer. */
        MPI_Request request;
        ret = MPI_Irecv(buffer, elems_to_swap * sizeof(*buffer),
                MPI_UNSIGNED_CHAR, remote_rank, local_idx, MPI_COMM_WORLD,
                &request);
        if (ret) {
            handle_error_string("Error receiving elem bytes");
            return;
        }

        /* Send local elems to the remote. */
        ret = MPI_Send(arr + local_idx - local_start,
                elems_to_swap * sizeof(*arr), MPI_UNSIGNED_CHAR, remote_rank,
                remote_idx, MPI_COMM_WORLD);
        if (ret) {
            handle_error_string("Error sending elem bytes");
            return;
        }

        /* Wait for received elems to come in. */
        ret = MPI_Wait(&request, MPI_STATUS_IGNORE);
        if (ret) {
            handle_error_string("Error waiting on receive for elem bytes");
            return;
        }

        /* Replace the local elements with the received remote elements if
         * necessary. Assume we are sorting ascending. If the local index is
         * lower, then we swap if the local element is lower. Likewise, if the
         * local index is higher, than we swap if the local element is higher.
         * If descending, everything is reversed. */
        for (size_t i = 0; i < elems_to_swap; i++) {
            if (((local_idx < remote_idx)
                        == (arr[i + local_idx - local_start].key
                            > buffer[i].key))
                    != descending) {
            memcpy(&arr[i + local_idx - local_start], &buffer[i],
                    sizeof(*arr));
            }
        }

        /* Bump pointers, decrement count, and continue. */
        local_idx += elems_to_swap;
        remote_idx += elems_to_swap;
        count -= elems_to_swap;
    }
}

static void swap(elem_t *arr, size_t a, size_t b, bool descending) {
    int a_rank = get_index_address(a);
    int b_rank = get_index_address(b);

    if (a_rank == world_rank && b_rank == world_rank) {
        swap_local_range(arr, a, b, 1, descending);
    } else if (a_rank == world_rank) {
        swap_remote_range(arr, a, b, 1, descending);
    } else if (b_rank == world_rank) {
        swap_remote_range(arr, b, a, 1, descending);
    }
}

static void swap_range(elem_t *arr, size_t a_start, size_t b_start,
        size_t count, bool descending) {
    size_t local_start = get_local_start(world_rank);
    size_t local_end = get_local_start(world_rank + 1);
    bool a_is_local = a_start < local_end && a_start + count > local_start;
    bool b_is_local = b_start < local_end && b_start + count > local_start;

    if (a_is_local && b_is_local) {
        swap_local_range(arr, a_start, b_start, count, descending);
    } else if (a_is_local) {
        size_t a_local_start = MAX(a_start, local_start);
        size_t a_local_end = MIN(a_start + count, local_end);
        swap_remote_range(arr, a_local_start, b_start + a_local_start - a_start,
                a_local_end - a_local_start, descending);
    } else if (b_is_local) {
        size_t b_local_start = MAX(b_start, local_start);
        size_t b_local_end = MIN(b_start + count, local_end);
        swap_remote_range(arr, b_local_start, a_start + b_local_start - b_start,
                b_local_end - b_local_start, descending);
    }
}

/* Bitonic sort. */

static void bitonic_merge(elem_t *arr, size_t start, size_t length,
        bool descending) {
    switch (length) {
        case 0:
        case 1:
            /* Do nothing. */
            break;
        case 2: {
            swap(arr, start, start + 1, descending);
            break;
        }
        default: {
            /* If the length is odd, bubble sort an element to the end of the
             * array and leave it there. */
            size_t left_length = length / 2;
            size_t right_length = length - left_length;
            size_t right_start = start + left_length;
            swap_range(arr, start, right_start, left_length, descending);
            if (right_start >= get_local_start(world_rank + 1)) {
                /* Only merge the left. The right is completely remote. */
                bitonic_merge(arr, start, left_length, descending);
            } else if (right_start <= get_local_start(world_rank)) {
                /* Only merge the right. The left is completely remote. */
                bitonic_merge(arr, right_start, right_length, descending);
            } else {
                /* Merge both. */
                bitonic_merge(arr, start, left_length, descending);
                bitonic_merge(arr, right_start, right_length, descending);
            }
            break;
         }
    }
}

void bitonic_sort(elem_t *arr, size_t start, size_t length,
        bool descending) {
    switch (length) {
        case 0:
        case 1:
            /* Do nothing. */
            break;
        case 2: {
            swap(arr, start, start + 1, descending);
            break;
        }
        default: {
            /* Sort left half forwards and right half in reverse to create a
             * bitonic sequence. */
            size_t left_length = length / 2;
            size_t right_length = length - left_length;
            size_t right_start = start + left_length;
            if (right_start >= get_local_start(world_rank + 1)) {
                /* Only sort the left. The right is completely remote. */
                bitonic_sort(arr, start, left_length, descending);
            } else if (right_start <= get_local_start(world_rank)) {
                /* Only sort the right. The left is completely remote. */
                bitonic_sort(arr, right_start, right_length, !descending);
            } else {
                /* Sort both. */
                bitonic_sort(arr, start, left_length, descending);
                bitonic_sort(arr, right_start, right_length, !descending);
            }

            /* Bitonic merge. */
            bitonic_merge(arr, start, length, descending);
            break;
        }
    }
}

int main(int argc, char **argv) {
    int ret = 0;

    /* Parse args. */
    if (argc < 2) {
        printf("usage: %s array_size\n", argv[0]);
        return -1;
    }
    ssize_t slength = atoll(argv[1]);
    if (slength < 0 || !is_pow2(slength)) {
        printf("Invalid array size\n");
        return -1;
    }
    size_t length = slength;

    ret = init_mpi(&argc, &argv, &world_rank, &world_size);
    if (ret) {
        handle_error_string("Error in MPI initialization");
        goto exit;
    }

    total_length = length;
    local_length = length / world_size;

    /* Allocate array. */
    elem_t *arr = calloc(local_length, sizeof(elem_t));
    if (!arr) {
        perror("alloc array");
        ret = errno;
        goto exit_finalize_mpi;
    }

    /* Allocate buffer. */
    buffer = malloc(SWAP_CHUNK_SIZE * sizeof(*buffer));
    if (!buffer) {
        perror("alloc buffer");
        ret = errno;
        goto exit_free_arr;
    }

    /* Add random elements to array. */
    srand(world_rank + 1);
    for (size_t i = 0; i < local_length; i++) {
        arr[i].key = rand();
    }

    /* Sort and time. */
    struct timespec start;
    timespec_get(&start, TIME_UTC);
    bitonic_sort(arr, 0, length, false);
    ret = MPI_Barrier(MPI_COMM_WORLD);
    if (ret) {
        handle_mpi_error(ret, "MPI_Barrier");
        goto exit_free_buffer;
    }
    struct timespec end;
    timespec_get(&end, TIME_UTC);

    /* Print time taken. */
    if (world_rank == 0) {
        double seconds_taken =
            (double) ((end.tv_sec * 1000000000 + end.tv_nsec)
                    - (start.tv_sec * 1000000000 + start.tv_nsec))
            / 1000000000;
        printf("%f\n", seconds_taken);
    }

exit_free_buffer:
    free(buffer);
exit_free_arr:
    free(arr);
exit_finalize_mpi:
    ret = MPI_Finalize();
    if (ret) {
        handle_mpi_error(ret, "MPI_Barrier");
    }
exit:
    return ret;
}
