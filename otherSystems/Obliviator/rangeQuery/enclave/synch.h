#ifndef DISTRIBUTED_SGX_SORT_ENCLAVE_SYNCH_H
#define DISTRIBUTED_SGX_SORT_ENCLAVE_SYNCH_H

#include <stdbool.h>

typedef struct spinlock {
    volatile bool locked;
} spinlock_t;

void spinlock_init(spinlock_t *lock);
void spinlock_lock(spinlock_t *lock);
bool spinlock_trylock(spinlock_t *lock);
void spinlock_unlock(spinlock_t *lock);

typedef struct sema {
    volatile unsigned int value;
} sema_t;

void sema_init(sema_t *sema, unsigned int initial_value);
void sema_up(sema_t *sema);
void sema_down(sema_t *sema);

typedef struct condvar {
    struct condvar_waiter *head;
    struct condvar_waiter *tail;
} condvar_t;

void condvar_init(condvar_t *condvar);
void condvar_wait(condvar_t *condvar, spinlock_t *lock);
void condvar_signal(condvar_t *condvar, spinlock_t *lock);
void condvar_broadcast(condvar_t *condvar, spinlock_t *lock);

#endif /* distributed-sgx-sort/enclave/synch.h */
