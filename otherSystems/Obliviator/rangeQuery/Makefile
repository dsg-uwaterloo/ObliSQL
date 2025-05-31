include config.mk

APP_NAME = parallel


COMMON_DIR = common
COMMON_OBJS = \
	$(COMMON_DIR)/error.o \
	$(COMMON_DIR)/util.o
COMMON_DEPS = $(COMMON_OBJS:.o=.d)

HOST_DIR = host
HOST_TARGET = $(HOST_DIR)/parallel
HOST_OBJS = \
	$(HOST_DIR)/parallel.o \
	$(HOST_DIR)/error.o \
	$(HOST_DIR)/ocalls.o
HOST_DEPS = $(HOST_OBJS:.o=.d)

ENCLAVE_DIR = enclave
ENCLAVE_TARGET = $(ENCLAVE_DIR)/parallel_enc
ENCLAVE_OBJS = \
	$(ENCLAVE_DIR)/parallel_enc.o \
	$(ENCLAVE_DIR)/crypto.o \
	$(ENCLAVE_DIR)/mpi_tls.o \
	$(ENCLAVE_DIR)/synch.o \
	$(ENCLAVE_DIR)/threading.o \
	$(ENCLAVE_DIR)/window.o \
	$(ENCLAVE_DIR)/scalable_oblivious_join.o \
	$(ENCLAVE_DIR)/oblivious_compact.o
ENCLAVE_DEPS = $(ENCLAVE_OBJS:.o=.d)
ENCLAVE_KEY = $(ENCLAVE_DIR)/$(APP_NAME).pem
ENCLAVE_PUBKEY = $(ENCLAVE_KEY:.pem=.pub)
ENCLAVE_CONF = $(ENCLAVE_DIR)/$(APP_NAME).conf

HOSTONLY_TARGET = hostonly
HOSTONLY_DEP = $(HOSTONLY_TARGET:=.d)

BASELINE_DIR = baselines
BASELINE_TARGETS = \
	$(BASELINE_DIR)/bitonic
BASELINE_DEPS = $(BASELINE_TARGETS:=.d)

LIBOBLIVIOUS = third_party/liboblivious
LIBOBLIVIOUS_LIB = $(LIBOBLIVIOUS)/liboblivious.a
THIRD_PARTY_LIBS = $(LIBOBLIVIOUS_LIB)

CPPFLAGS = -I. \
	-I$(LIBOBLIVIOUS)/include
CFLAGS = -march=native -mno-avx512f -O3 -Wall -Wextra -Werror  -DOE_SIMULATION_CERT
LDFLAGS = \
	-L$(LIBOBLIVIOUS)
LDLIBS = \
	-l:liboblivious.a

# all target.

.PHONY: all
all: $(HOST_TARGET) $(ENCLAVE_TARGET).signed

# SGX edge.

HOST_EDGE_HEADERS = $(HOST_DIR)/$(APP_NAME)_u.h $(HOST_DIR)/$(APP_NAME)_args.h
HOST_EDGE_SRC = $(HOST_DIR)/$(APP_NAME)_u.c
HOST_EDGE_OBJS = $(HOST_EDGE_SRC:.c=.o)
ENCLAVE_EDGE_HEADERS = $(ENCLAVE_DIR)/$(APP_NAME)_t.h $(ENCLAVE_DIR)/$(APP_NAME)_args.h
ENCLAVE_EDGE_SRC = $(ENCLAVE_DIR)/$(APP_NAME)_t.c
ENCLAVE_EDGE_OBJS = $(ENCLAVE_EDGE_SRC:.c=.o)
SGX_EDGE = $(HOST_EDGE_HEADERS) $(HOST_EDGE_SRC) $(ENCLAVE_EDGE_HEADERS) $(ENCLAVE_EDGE_SRC)

INCDIR = $(shell pkg-config oehost-$(C_COMPILER) --variable=includedir)
$(SGX_EDGE): $(APP_NAME).edl
	$(SGX_EDGER8R) $< \
		--untrusted-dir $(HOST_DIR) \
		--trusted-dir $(ENCLAVE_DIR) \
		--search-path $(INCDIR) \
		--search-path $(INCDIR)/openenclave/edl/sgx

# Dependency generation.

CPPFLAGS += -MMD

%.d: $(SGX_EDGE);

# Third-party deps.

$(LIBOBLIVIOUS_LIB):
	$(MAKE) -C $(LIBOBLIVIOUS)

# Host.

HOST_CPPFLAGS = $(CPPFLAGS)
HOST_CFLAGS = \
	$(shell pkg-config mpi --cflags) \
	$(shell pkg-config oehost-$(C_COMPILER) --cflags) \
	$(CFLAGS)
HOST_LDFLAGS = $(LDFLAGS)
HOST_LDLIBS = \
	$(shell pkg-config mpi --libs) \
	$(shell pkg-config oehost-$(C_COMPILER) --libs) \
	-lmbedcrypto \
	$(LDLIBS)

$(HOST_DIR)/%.o: $(HOST_DIR)/%.c
	$(CC) $(HOST_CFLAGS) $(HOST_CPPFLAGS) -c -o $@ $<

$(HOST_TARGET): $(HOST_OBJS) $(HOST_EDGE_OBJS) $(COMMON_OBJS) $(THIRD_PARTY_LIBS)
	$(CC) $(HOST_LDFLAGS) $(HOST_OBJS) $(HOST_EDGE_OBJS) $(COMMON_OBJS) $(HOST_LDLIBS) -o $@

# Enclave.

ENCLAVE_CPPFLAGS = $(CPPFLAGS)
ENCLAVE_CFLAGS = \
	$(shell pkg-config oeenclave-$(C_COMPILER) --cflags) \
	$(CFLAGS)
ENCLAVE_LDFLAGS = $(LDFLAGS)
ENCLAVE_LDLIBS = \
	$(shell pkg-config oeenclave-$(C_COMPILER) --libs) \
	$(shell pkg-config oeenclave-$(C_COMPILER) --variable=mbedtlslibs) \
	$(LDLIBS)

$(ENCLAVE_DIR)/%.o: $(ENCLAVE_DIR)/%.c
	$(CC) $(ENCLAVE_CFLAGS) $(ENCLAVE_CPPFLAGS) -c -o $@ $<

$(ENCLAVE_TARGET): $(ENCLAVE_OBJS) $(ENCLAVE_EDGE_OBJS) $(COMMON_OBJS) $(THIRD_PARTY_LIBS)
	$(CC) $(ENCLAVE_LDFLAGS) $(ENCLAVE_OBJS) $(ENCLAVE_EDGE_OBJS) $(COMMON_OBJS) $(ENCLAVE_LDLIBS) -o $@

$(ENCLAVE_TARGET).signed: $(ENCLAVE_TARGET) $(ENCLAVE_KEY) $(ENCLAVE_PUBKEY) $(ENCLAVE_CONF)
	$(SGX_SIGN) sign -e $< -k $(ENCLAVE_KEY) -c $(ENCLAVE_CONF)

$(ENCLAVE_KEY):
	openssl genrsa -out $@ -3 3072

$(ENCLAVE_PUBKEY): $(ENCLAVE_KEY)
	openssl rsa -in $< -pubout -out $@

# Common.

$(COMMON_DIR)/%.o: $(COMMON_DIR)/%.c
	$(CC) $(HOST_CFLAGS) $(HOST_CPPFLAGS) -c -o $@ $<

# Host-only binary for profiling.

HOSTONLY_CPPFLAGS = \
	-DDISTRIBUTED_SGX_SORT_HOSTONLY \
	$(CPPFLAGS)
HOSTONLY_CFLAGS = \
	$(shell pkg-config mpi --cflags) \
	-Wno-implicit-function-declaration -Wno-unused \
	$(CFLAGS)
HOSTONLY_LDFLAGS = $(LDFLAGS)
HOSTONLY_LDLIBS = \
	$(shell pkg-config mpi --libs) \
	-lmbedcrypto \
	-lmbedx509 \
	-lmbedtls \
	$(LDLIBS)

$(HOSTONLY_TARGET): $(HOST_OBJS:.o=.c) $(ENCLAVE_OBJS:.o=.c) $(COMMON_OBJS:.o=.c) $(THIRD_PARTY_LIBS)
	$(CC) $(HOSTONLY_CFLAGS) $(HOSTONLY_CPPFLAGS) $(HOSTONLY_LDFLAGS) $(HOST_OBJS:.o=.c) $(ENCLAVE_OBJS:.o=.c) $(COMMON_OBJS:.o=.c) $(HOSTONLY_LDLIBS) -o $@

# Baselines.

BASELINE_CPPFLAGS = $(HOST_CPPFLAGS)
BASELINE_CFLAGS = $(HOST_CFLAGS)
BASELINE_LDFLAGS = $(HOST_LDFLAGS)
BASELINE_LDLIBS = $(HOST_LDLIBS)

$(BASELINE_DIR)/%: $(BASELINE_DIR)/%.c $(HOST_DIR)/error.o $(COMMON_OBJS:.o=.c) $(THIRD_PARTY_LIBS)
	$(CC) $(BASELINE_CFLAGS) $(BASELINE_CPPFLAGS) $(BASELINE_LDFLAGS) $< $(HOST_DIR)/error.o $(COMMON_OBJS:.o=.c) $(BASELINE_LDLIBS) -o $@

# Misc.

.PHONY: clean
clean:
	$(MAKE) -C $(LIBOBLIVIOUS) clean
	rm -f $(SGX_EDGE) \
		$(COMMON_DEPS) $(COMMON_OBJS) \
		$(HOST_TARGET) $(HOST_DEPS) $(HOST_OBJS) \
		$(ENCLAVE_TARGET).signed $(ENCLAVE_TARGET) $(ENCLAVE_DEPS) $(ENCLAVE_OBJS) \
		$(ENCLAVE_PUBKEY) $(ENCLAVE_KEY) \
		$(HOSTONLY_TARGET) $(HOSTONLY_DEP) \
		$(BASELINE_TARGETS) $(BASELINE_DEPS)

-include $(COMMON_DEPS)
-include $(HOST_DEPS)
-include $(ENCLAVE_DEPS)
-include $(HOSTONLY_DEP)
-include $(BASELINE_DEPS)
