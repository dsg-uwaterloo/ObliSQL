#include "enclave/synch.h"
#include <stddef.h>
#include "common/defs.h"

#define ADAPTIVE_TIMEOUT 10000

#define PAUSE() asm("pause")

void spinlock_init(spinlock_t *lock) {
    lock->locked = false;
}

void spinlock_lock(spinlock_t *lock) {
    while (lock->locked || __atomic_test_and_set(&lock->locked, __ATOMIC_ACQUIRE)) {
        PAUSE();
    }
}

bool spinlock_trylock(spinlock_t *lock) {
    return
        !lock->locked
        && !__atomic_test_and_set(&lock->locked, __ATOMIC_ACQUIRE);
}

void spinlock_unlock(spinlock_t *lock) {
    __atomic_clear(&lock->locked, __ATOMIC_RELEASE);
}

void sema_init(sema_t *sema, unsigned int initial_value) {
    sema->value = initial_value;
}

void sema_up(sema_t *sema) {
    __atomic_add_fetch(&sema->value, 1, __ATOMIC_ACQUIRE);
}

void sema_down(sema_t *sema) {
    unsigned int val;
    size_t spin_count = 0;
    do {
        spin_count++;
        PAUSE();
        val = sema->value;
    } while (!val || !__atomic_compare_exchange_n(&sema->value, &val, val - 1,
                false, __ATOMIC_RELEASE, __ATOMIC_RELAXED));
}

struct condvar_waiter {
    sema_t sema;
    struct condvar_waiter *next;
};

void condvar_init(condvar_t *condvar) {
    condvar->head = NULL;
    condvar->tail = NULL;
}

void condvar_wait(condvar_t *condvar, spinlock_t *lock) {
    struct condvar_waiter waiter;
    sema_init(&waiter.sema, 0);
    waiter.next = NULL;
    if (condvar->tail) {
        condvar->tail->next = &waiter;
    } else {
        condvar->head = &waiter;
    }
    condvar->tail = &waiter;
    spinlock_unlock(lock);
    sema_down(&waiter.sema);
    spinlock_lock(lock);
}

void condvar_signal(condvar_t *condvar, spinlock_t *lock UNUSED) {
    if (condvar->head) {
        sema_up(&condvar->head->sema);
        if (condvar->head == condvar->tail) {
            condvar->tail = NULL;
        }
        condvar->head = condvar->head->next;
    }
}

void condvar_broadcast(condvar_t *condvar, spinlock_t *lock UNUSED) {
    while (condvar->head) {
        sema_up(&condvar->head->sema);
        if (condvar->head == condvar->tail) {
            condvar->tail = NULL;
        }
        condvar->head = condvar->head->next;
    }
}
