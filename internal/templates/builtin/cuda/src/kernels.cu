#include "cuda_check.cuh"
#include "kernels.cuh"

__global__ void vector_add(const float* a, const float* b, float* c, int n) {
    int i = blockIdx.x * blockDim.x + threadIdx.x;
    if (i < n) {
        c[i] = a[i] + b[i];
    }
}

void launch_vector_add(const float* a, const float* b, float* c, int n) {
    const int threads = 256;
    const int blocks = (n + threads - 1) / threads;
    vector_add<<<blocks, threads>>>(a, b, c, n);
    CUDA_CHECK(cudaGetLastError());
}
