#pragma once

#include <stddef.h>
#include <stdint.h>

#include "faceproof_biometric_vision.h"

#ifdef __cplusplus
extern "C" {
#endif

#define FP_SECURE_CORE_ABI_VERSION 1
#define FP_SECURE_CORE_VERSION "0.2.1"
#define FP_SECURE_CORE_EMBEDDING_CAPACITY 512
#define FP_SECURE_CORE_MAX_SELECTED_FRAMES 3

typedef struct FPSecureCore FPSecureCore;

typedef enum FPSecureCorePhase {
    FP_SECURE_PHASE_FAR = 0,
    FP_SECURE_PHASE_NEAR = 1
} FPSecureCorePhase;

typedef struct FPSecureCoreFrameInput {
    const uint8_t* data;
    size_t size;
    int32_t phase;
} FPSecureCoreFrameInput;

typedef struct FPSecureCoreSelectedEmbedding {
    int32_t frame_index;
    int32_t phase;
    double quality;
    size_t embedding_size;
    float embedding[FP_SECURE_CORE_EMBEDDING_CAPACITY];
} FPSecureCoreSelectedEmbedding;

typedef struct FPSecureCoreIdentityResult {
    double liveness_score;
    double passive_pad_score;
    int32_t passive_pad_available;
    double temporal_motion_score;
    double guided_capture_score;
    double quality_score;
    double face_presence;
    double sharpness;
    double brightness;
    double face_size;
    int32_t detected_frames;
    int32_t processed_frames;
    int32_t best_frame_index;
    size_t embedding_size;
    float embedding[FP_SECURE_CORE_EMBEDDING_CAPACITY];
    size_t selected_embedding_count;
    FPSecureCoreSelectedEmbedding selected_embeddings[
        FP_SECURE_CORE_MAX_SELECTED_FRAMES
    ];
} FPSecureCoreIdentityResult;

typedef struct FPSecureCoreReferenceResult {
    size_t embedding_size;
    size_t variant_count;
    float embedding[FP_SECURE_CORE_EMBEDDING_CAPACITY];
    float variants[
        FP_SECURE_CORE_MAX_SELECTED_FRAMES
    ][FP_SECURE_CORE_EMBEDDING_CAPACITY];
    FPBiometricFace face;
    FPBiometricQuality quality;
} FPSecureCoreReferenceResult;

typedef struct FPSecureCoreGuideResult {
    int32_t face_detected;
    int32_t image_width;
    int32_t image_height;
    double confidence;
    double center_x;
    double center_y;
    double width_ratio;
    double height_ratio;
    double roll_degrees;
    FPBiometricQuality quality;
} FPSecureCoreGuideResult;

int fp_secure_core_abi_version(void);
const char* fp_secure_core_version(void);
const char* fp_secure_core_last_error(void);

FPSecureCore* fp_secure_core_create(
    const char* yunet_model_path,
    const char* sface_model_path,
    const char* minifasnet_model_path
);

void fp_secure_core_destroy(FPSecureCore* core);

int fp_secure_core_analyze_identity(
    FPSecureCore* core,
    const FPSecureCoreFrameInput* frames,
    size_t frame_count,
    FPSecureCoreIdentityResult* result
);

int fp_secure_core_reference_jpeg(
    FPSecureCore* core,
    const uint8_t* jpeg_data,
    size_t jpeg_size,
    FPSecureCoreReferenceResult* result
);

int fp_secure_core_guide_jpeg(
    FPSecureCore* core,
    const uint8_t* jpeg_data,
    size_t jpeg_size,
    FPSecureCoreGuideResult* result
);

#ifdef __cplusplus
}
#endif
