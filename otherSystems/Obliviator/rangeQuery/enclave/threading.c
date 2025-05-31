#include "enclave/threading.h"
#include <stdbool.h>
#include <stddef.h>
#include "enclave/synch.h"
#include "common/util.h"

struct task {
    struct thread_work *work;
    union {
        struct {} single;
        struct {
            size_t i;
        } iter;
    };
};

size_t total_num_threads;
size_t num_threads_working;

static spinlock_t thread_work_lock;
static struct thread_work *volatile work_head;
static struct thread_work *volatile work_tail;
static volatile bool work_done;

void thread_work_push(struct thread_work *work) {
    sema_init(&work->done, 0);

    switch (work->type) {
        case THREAD_WORK_SINGLE:
            // Do nothing.
            break;
        case THREAD_WORK_ITER:
            if (!work->iter.count) {
                sema_up(&work->done);
                return;
            }
            work->iter.curr = 0;
            work->iter.num_remaining = work->iter.count;
            break;
    }

    spinlock_lock(&thread_work_lock);
    work->next = NULL;
    if (!work_tail) {
        /* Empty list. Set head and tail. */
        work_head = work;
        work_tail = work;
    } else {
        /* List has values. */
        work_tail->next = work;
        work_tail = work;
    }
    spinlock_unlock(&thread_work_lock);
}

void thread_wait(struct thread_work *work) {
    sema_down(&work->done);
}

static bool get_task(struct task *task) {
    task->work = NULL;
    if (work_head) {
        spinlock_lock(&thread_work_lock);
        if (work_head) {
            task->work = work_head;

            bool pop_work = false;
            switch (task->work->type) {
                case THREAD_WORK_SINGLE:
                    pop_work = true;
                    break;
                case THREAD_WORK_ITER:
                    task->iter.i = task->work->iter.curr;
                    task->work->iter.curr++;
                    if (task->work->iter.curr
                            >= task->work->iter.count) {
                        pop_work = true;
                    }
                    break;
            }

            if (pop_work) {
                if (!work_head->next) {
                    work_tail = NULL;
                }
                work_head = work_head->next;
            }
        }
        spinlock_unlock(&thread_work_lock);
    }
    return task->work;
}

static void do_task(struct task *task) {
    //printf("[INFO] begin do task\n");
    switch (task->work->type) {
        case THREAD_WORK_SINGLE:
            task->work->single.func(task->work->single.arg);
            sema_up(&task->work->done);
            break;
        case THREAD_WORK_ITER:
#pragma GCC diagnostic push
#pragma GCC diagnostic ignored "-Wmaybe-uninitialized"
            task->work->iter.func(task->work->iter.arg,
                    task->iter.i);
#pragma GCC diagnostic pop
            if (!__atomic_sub_fetch(&task->work->iter.num_remaining, 1,
                        __ATOMIC_RELEASE)) {
                sema_up(&task->work->done);
            }
            break;
    }
    //printf("[INFO] end do task\n");
}

void thread_start_work(void) {
    __atomic_add_fetch(&num_threads_working, 1, __ATOMIC_ACQUIRE);

    while (!work_done) {
        struct task task;
        if (get_task(&task)) {
            do_task(&task);
        }
    }

    __atomic_sub_fetch(&num_threads_working, 1, __ATOMIC_RELEASE);
}

void thread_work_until_empty(void) {
    __atomic_add_fetch(&num_threads_working, 1, __ATOMIC_ACQUIRE);

    struct task task;
    while (get_task(&task)) {
        do_task(&task);
    }

    __atomic_sub_fetch(&num_threads_working, 1, __ATOMIC_RELEASE);
}

void thread_wait_for_all(void) {
    static size_t num_threads_waiting;
    static condvar_t all_threads_finished;
    static spinlock_t all_threads_lock;

    spinlock_lock(&all_threads_lock);
    num_threads_waiting++;
    if (num_threads_waiting >= total_num_threads) {
        condvar_broadcast(&all_threads_finished, &all_threads_lock);
        num_threads_waiting = 0;
    } else {
        condvar_wait(&all_threads_finished, &all_threads_lock);
    }
    spinlock_unlock(&all_threads_lock);
}

void thread_release_all(void) {
    work_done = true;
}

void thread_unrelease_all(void) {
    work_done = false;
}
