#pragma once

#include <cstdio>
#include <cstdlib>

#include <cuda_runtime.h>

// Wrap every CUDA runtime call: CUDA_CHECK(cudaMalloc(...));
#define CUDA_CHECK(call)                                                        \
    do {                                                                        \
        cudaError_t err_ = (call);                                              \
        if (err_ != cudaSuccess) {                                              \
            std::fprintf(stderr, "CUDA error %s:%d: %s\n", __FILE__, __LINE__,  \
                         cudaGetErrorString(err_));                             \
            std::exit(EXIT_FAILURE);                                            \
        }                                                                       \
    } while (0)
