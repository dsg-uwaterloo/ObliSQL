# Choose the right pkg-config based on CC.
ifneq ($(shell $(CC) --version | grep clang),)
	C_COMPILER = clang
else
	C_COMPILER = gcc
endif

SGX_EDGER8R ?= oeedger8r
SGX_SIGN ?= oesign
