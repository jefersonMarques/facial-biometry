#pragma once

#include <stddef.h>
#include <stdint.h>

#include "faceproof_biometric_vision.h"

#ifdef __cplusplus
extern "C" {
#endif

typedef struct FPBiometricPAD FPBiometricPAD;

typedef struct FPBiometricPADResult {
    double real_probability;
    int32_t input_width;
    int32_t input_height;
    int32_t class_count;
} FPBiometricPADResult;

FPBiometricPAD* fp_bio_pad_create(const char* model_path);
void fp_bio_pad_destroy(FPBiometricPAD* pad);
const char* fp_bio_pad_last_error(void);

int fp_bio_pad_predict_jpeg(
    FPBiometricPAD* pad,
    const uint8_t* image_data,
    size_t image_size,
    const FPBiometricFace* face,
    FPBiometricPADResult* result
);

#ifdef __cplusplus
}
#endif
