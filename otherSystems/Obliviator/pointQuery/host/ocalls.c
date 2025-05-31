#include <errno.h>
#include <limits.h>
#include <stddef.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <mpi.h>
#include <time.h>
#include "common/defs.h"
#include "common/error.h"
#include "common/ocalls.h"
#include "host/error.h"
//#include "host/parallel.h"

struct timespec last_time;
struct timespec last_time2;

enum ocall_mpi_request_type {
    OCALL_MPI_SEND,
    OCALL_MPI_RECV,
};

struct ocall_mpi_request {
    enum ocall_mpi_request_type type;
    void *buf;
    MPI_Request mpi_request;
};

int ocall_mpi_send_bytes(const unsigned char *buf, size_t count, int dest,
        int tag) {
    if (count > INT_MAX) {
        handle_error_string("Count too large");
        return MPI_ERR_COUNT;
    }

    return MPI_Send(buf, (int) count, MPI_UNSIGNED_CHAR, dest, tag,
            MPI_COMM_WORLD);
}

int ocall_mpi_recv_bytes(unsigned char *buf, size_t count, int source,
        int tag, ocall_mpi_status_t *status) {
    int ret;

    if (count > INT_MAX) {
        handle_error_string("Count too large");
        ret = MPI_ERR_COUNT;
        goto exit;
    }

    if (source == OCALL_MPI_ANY_SOURCE) {
        source = MPI_ANY_SOURCE;
    }
    if (tag == OCALL_MPI_ANY_TAG) {
        tag = MPI_ANY_TAG;
    }

    MPI_Status mpi_status;
    ret = MPI_Recv(buf, (int) count, MPI_UNSIGNED_CHAR, source, tag,
            MPI_COMM_WORLD, &mpi_status);

    /* Populate status. */
    ret = MPI_Get_count(&mpi_status, MPI_UNSIGNED_CHAR, &status->count);
    if (ret) {
        handle_mpi_error(ret, "MPI_Get_count");
        goto exit;
    }
    status->source = mpi_status.MPI_SOURCE;
    status->tag = mpi_status.MPI_TAG;

exit:
    return ret;
}

int ocall_mpi_try_recv_bytes(unsigned char *buf, size_t count, int source,
        int tag, int *flag, ocall_mpi_status_t *status) {
    if (count > INT_MAX) {
        handle_error_string("Count too large");
        return -1;
    }

    MPI_Status mpi_status;
    int ret;

    if (source == OCALL_MPI_ANY_SOURCE) {
        source = MPI_ANY_SOURCE;
    }
    if (tag == OCALL_MPI_ANY_TAG) {
        tag = MPI_ANY_TAG;
    }

    /* Probe for an available message. */
    ret = MPI_Iprobe(source, tag, MPI_COMM_WORLD, flag, &mpi_status);
    if (ret) {
        handle_mpi_error(ret, "MPI_Probe");
        goto exit;
    }
    if (!*flag) {
        goto exit;
    }

    /* Get incoming message parameters. */
    int bytes_to_recv;
    ret = MPI_Get_count(&mpi_status, MPI_UNSIGNED_CHAR, &bytes_to_recv);
    if (ret) {
        handle_mpi_error(ret, "MPI_Get_count");
        goto exit;
    }
    source = mpi_status.MPI_SOURCE;
    tag = mpi_status.MPI_TAG;

    /* Read in that number of bytes. */
    ret = MPI_Recv(buf, count, MPI_UNSIGNED_CHAR, source, tag,
            MPI_COMM_WORLD, MPI_STATUS_IGNORE);
    if (ret) {
        handle_mpi_error(ret, "MPI_Recv");
        goto exit;
    }

    /* Populate status. */
    status->count = bytes_to_recv;
    status->source = source;
    status->tag = tag;

exit:
    return ret;
}

int ocall_mpi_isend_bytes(const unsigned char *buf, size_t count, int dest,
        int tag, ocall_mpi_request_t *request) {
    int ret;

    if (count > INT_MAX) {
        handle_error_string("Count too large");
        return MPI_ERR_COUNT;
    }

    /* Allocate request. */
    *request = malloc(sizeof(**request));
    if (!*request) {
        perror("malloc ocall_mpi_request");
        ret = errno;
        goto exit;
    }
    (*request)->type = OCALL_MPI_SEND;
    (*request)->buf = malloc(count);
    if (!(*request)->buf) {
        perror("malloc isend buf");
        ret = errno;
        goto exit_free_request;
    }

    /* Copy bytes to send to permanent buffer. */
    memcpy((*request)->buf, buf, count);

    /* Start request. */
    ret = MPI_Isend((*request)->buf, (int) count, MPI_UNSIGNED_CHAR, dest, tag,
            MPI_COMM_WORLD, &(*request)->mpi_request);
    if (ret) {
        handle_mpi_error(ret, "MPI_Isend");
        goto exit_free_buf;
    }

    ret = 0;

    return ret;

exit_free_buf:
    free((*request)->buf);
exit_free_request:
    free(*request);
exit:
    return ret;
}

int ocall_mpi_irecv_bytes(size_t count, int source, int tag,
        ocall_mpi_request_t *request) {
    int ret;

    if (count > INT_MAX) {
        handle_error_string("Count too large");
        return MPI_ERR_COUNT;
    }

    if (source == OCALL_MPI_ANY_SOURCE) {
        source = MPI_ANY_SOURCE;
    }
    if (tag == OCALL_MPI_ANY_TAG) {
        tag = MPI_ANY_TAG;
    }

    /* Allocate request. */
    *request = malloc(sizeof(**request));
    if (!*request) {
        perror("malloc ocall_mpi_request");
        ret = errno;
        goto exit;
    }
    (*request)->type = OCALL_MPI_RECV;
    (*request)->buf = malloc(count);
    if (!(*request)->buf) {
        perror("malloc irecv buf");
        ret = errno;
        goto exit_free_request;
    }

    /* Start request. */
    ret = MPI_Irecv((*request)->buf, (int) count, MPI_UNSIGNED_CHAR, source,
            tag, MPI_COMM_WORLD, &(*request)->mpi_request);
    if (ret) {
        handle_mpi_error(ret, "MPI_Irecv");
        goto exit_free_buf;
    }

    ret = 0;

    return ret;

exit_free_buf:
    free((*request)->buf);
exit_free_request:
    free(*request);
exit:
    return ret;
}

int ocall_mpi_wait(unsigned char *buf, size_t count,
        ocall_mpi_request_t *request, ocall_mpi_status_t *status) {
    int ret;

    MPI_Request mpi_request;
    if (*request == OCALL_MPI_REQUEST_NULL) {
        mpi_request = MPI_REQUEST_NULL;
    } else {
        mpi_request = (*request)->mpi_request;
    }

    MPI_Status mpi_status;
    ret = MPI_Wait(&mpi_request, &mpi_status);
    if (ret) {
        handle_mpi_error(ret, "MPI_Wait");
        goto exit_free_request;
    }

    switch ((*request)->type) {
    case OCALL_MPI_SEND:
        break;

    case OCALL_MPI_RECV:
        /* Populate status. */
        ret = MPI_Get_count(&mpi_status, MPI_UNSIGNED_CHAR, &status->count);
        if (ret) {
            handle_mpi_error(ret, "MPI_Get_count");
            goto exit_free_request;
        }
        status->source = mpi_status.MPI_SOURCE;
        status->tag = mpi_status.MPI_TAG;

        /* Copy bytes to output. */
        memcpy(buf, (*request)->buf, MIN(count, (size_t) status->count));

        break;
    }

exit_free_request:
    free((*request)->buf);
    free(*request);
    return ret;
}

int ocall_mpi_waitany(unsigned char *buf, size_t bufcount, size_t count,
        ocall_mpi_request_t *requests, size_t *index,
        ocall_mpi_status_t *status) {
    int ret;

    MPI_Request mpi_requests[count];
    for (size_t i = 0; i < count; i++) {
        if (requests[i] == OCALL_MPI_REQUEST_NULL) {
            mpi_requests[i] = MPI_REQUEST_NULL;
        } else {
            mpi_requests[i] = requests[i]->mpi_request;
        }
    }

    MPI_Status mpi_status;
    int mpi_index;
    ret = MPI_Waitany(count, mpi_requests, &mpi_index, &mpi_status);
    if (ret) {
        handle_mpi_error(ret, "MPI_Waitany");
        goto exit_free_request;
    }
    if (mpi_index == MPI_UNDEFINED) {
        ret = -1;
        handle_error_string("All null requests passed to ocall_mpi_waitany");
        goto exit;
    }
    *index = mpi_index;

    switch (requests[*index]->type) {
    case OCALL_MPI_SEND:
        break;

    case OCALL_MPI_RECV:
        /* Populate status. */
        ret = MPI_Get_count(&mpi_status, MPI_UNSIGNED_CHAR, &status->count);
        if (ret) {
            handle_mpi_error(ret, "MPI_Get_count");
            goto exit_free_request;
        }
        status->source = mpi_status.MPI_SOURCE;
        status->tag = mpi_status.MPI_TAG;

        /* Copy bytes to output. */
        memcpy(buf, requests[*index]->buf, MIN(bufcount,
                    (size_t) status->count));

        break;
    }

exit_free_request:
    free(requests[*index]->buf);
    free(requests[*index]);
exit:
    return ret;
}

int ocall_mpi_try_wait(unsigned char *buf, size_t count,
        ocall_mpi_request_t *request, int *flag, ocall_mpi_status_t *status) {
    int ret;

    MPI_Status mpi_status;

    /* Test request status. */
    ret = MPI_Test(&(*request)->mpi_request, flag, &mpi_status);
    if (ret) {
        handle_mpi_error(ret, "MPI_Test");
        goto exit_free_request;
    }
    if (!*flag) {
        goto exit;
    }

    switch ((*request)->type) {
    case OCALL_MPI_SEND:
        break;

    case OCALL_MPI_RECV:
        /* Populate status. */
        ret = MPI_Get_count(&mpi_status, MPI_UNSIGNED_CHAR, &status->count);
        if (ret) {
            handle_mpi_error(ret, "MPI_Get_count");
            goto exit_free_request;
        }
        status->source = mpi_status.MPI_SOURCE;
        status->tag = mpi_status.MPI_TAG;

        /* Copy bytes to output. */
        memcpy(buf, (*request)->buf, MIN(count, (size_t) status->count));

        break;
    }

exit_free_request:
    free((*request)->buf);
    free(*request);
exit:
    return ret;
}

int ocall_mpi_cancel(ocall_mpi_request_t *request) {
    int ret;

    ret = MPI_Cancel(&(*request)->mpi_request);
    if (ret) {
        handle_mpi_error(ret, "MPI_Cancel");
        goto exit_free_request;
    }

exit_free_request:
    free((*request)->buf);
    free(*request);
    return ret;
}

void ocall_mpi_barrier(void) {
    MPI_Barrier(MPI_COMM_WORLD);
}

void ocall_print_string(const char *str)
{
    /* Proxy/Bridge will check the length and null-terminate 
     * the input string to prevent buffer overflow. 
     */
    printf("%s", str);
}

void init_time(void) {
    int ret;
    ret = timespec_get(&last_time, TIME_UTC);
    if (!ret) {
        perror("ending timespec_get");
    }
    return;
}

void get_time(bool if_print) {
    
    struct timespec end;
    
    int ret;
    ret = timespec_get(&end, TIME_UTC);
    if (!ret) {
        perror("ending timespec_get");
    }

    /* Print time taken. */
    //if (world_rank == 0) {
        double seconds_taken =
                (double) ((end.tv_sec * 1000000000 + end.tv_nsec)
                - (last_time.tv_sec * 1000000000 + last_time.tv_nsec))
                / 1000000000;
        if (if_print) {
            printf("%f\n", seconds_taken);
        }
    //}

    last_time.tv_sec = end.tv_sec;
    last_time.tv_nsec = end.tv_nsec;
    return;
}

void init_time2(void) {
    int ret;
    ret = timespec_get(&last_time2, TIME_UTC);
    if (!ret) {
        perror("ending timespec_get");
    }
    return;
}

void get_time2(bool if_print) {
    
    struct timespec end;
    
    int ret;
    ret = timespec_get(&end, TIME_UTC);
    if (!ret) {
        perror("ending timespec_get");
    }

    /* Print time taken. */
    //if (world_rank == 0) {
        double seconds_taken =
                (double) ((end.tv_sec * 1000000000 + end.tv_nsec)
                - (last_time2.tv_sec * 1000000000 + last_time2.tv_nsec))
                / 1000000000;
        if (if_print) {
            printf("%f\n", seconds_taken);
        }
    //}

    last_time2.tv_sec = end.tv_sec;
    last_time2.tv_nsec = end.tv_nsec;
    return;
}
