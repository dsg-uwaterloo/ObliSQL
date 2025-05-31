#ifndef DISTRIBUTED_SGX_SORT_ENCLAVE_THREADING_H
#define DISTRIBUTED_SGX_SORT_ENCLAVE_THREADING_H

#include <stdbool.h>
#include <stddef.h>
#include "enclave/synch.h"

enum thread_work_type {
    THREAD_WORK_SINGLE,
    THREAD_WORK_ITER,
};

struct thread_work {
    enum thread_work_type type;
    union {
        struct {
            void (*func)(void *arg);
            void *arg;
        } single;
        struct {
            void (*func)(void *arg, size_t i);
            void *arg;
            size_t count;

            size_t num_remaining;
            size_t curr;
        } iter;
    };

    sema_t done;

    struct thread_work *next;
};

extern size_t total_num_threads;
extern size_t num_threads_working;

void thread_work_push(struct thread_work *work);
void thread_wait(struct thread_work *work);
void thread_start_work(void);
void thread_work_until_empty(void);
void thread_wait_for_all(void);
void thread_release_all(void);
void thread_unrelease_all(void);

#endif /* distributed-sgx-sort/enclave/threading.h */
