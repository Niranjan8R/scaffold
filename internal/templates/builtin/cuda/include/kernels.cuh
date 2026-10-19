#pragma once

// Computes c[i] = a[i] + b[i] on the GPU. All pointers are device pointers.
void launch_vector_add(const float* a, const float* b, float* c, int n);
