#pragma once

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct FPBiometricVisionEngine FPBiometricVisionEngine;

typedef struct FPBiometricFace {
    int32_t detected;
    int32_t x;
    int32_t y;
    int32_t width;
    int32_t height;
    double confidence;
    float raw[15];
} FPBiometricFace;

typedef struct FPBiometricQuality {
    double sharpness;
    double brightness;
    double brightness_value;
    double face_size;
    double score;
} FPBiometricQuality;

FPBiometricVisionEngine* fp_bio_vision_create(
    const char* yunet_model_path,
    const char* sface_model_path
);

void fp_bio_vision_destroy(FPBiometricVisionEngine* engine);

int fp_bio_vision_analyze_jpeg(
    FPBiometricVisionEngine* engine,
    const uint8_t* jpeg_data,
    size_t jpeg_size,
    FPBiometricFace* face,
    FPBiometricQuality* quality
);

int fp_bio_vision_encode_jpeg(
    FPBiometricVisionEngine* engine,
    const uint8_t* jpeg_data,
    size_t jpeg_size,
    float* embedding,
    size_t embedding_capacity,
    size_t* embedding_size,
    FPBiometricFace* face,
    FPBiometricQuality* quality
);

#ifdef __cplusplus
}
#endif
