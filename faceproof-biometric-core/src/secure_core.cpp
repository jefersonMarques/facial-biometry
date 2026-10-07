#include "faceproof_secure_core.h"

#include "faceproof_biometric.h"
#include "faceproof_biometric_pad.h"

#include <opencv2/core.hpp>
#include <opencv2/imgcodecs.hpp>
#include <opencv2/imgproc.hpp>

#include <algorithm>
#include <cmath>
#include <cstring>
#include <memory>
#include <mutex>
#include <sstream>
#include <string>
#include <utility>
#include <vector>

struct FPSecureCore {
    FPBiometricVisionEngine* vision = nullptr;
    FPBiometricPAD* pad = nullptr;
    std::mutex mutex;
};

namespace {

thread_local std::string g_last_error;

constexpr double kCropMargin = 0.08;

void set_last_error(const std::string& value) {
    g_last_error = value;
}

double clamp01(double value) {
    return std::max(0.0, std::min(1.0, value));
}

cv::Mat decode_jpeg(const uint8_t* data, size_t size) {
    if (data == nullptr || size == 0) {
        return {};
    }
    const cv::Mat encoded(
        1,
        static_cast<int>(size),
        CV_8UC1,
        const_cast<uint8_t*>(data)
    );
    return cv::imdecode(encoded, cv::IMREAD_COLOR);
}

cv::Mat crop_face(const cv::Mat& image, const FPBiometricFace& face) {
    const int margin_x = static_cast<int>(
        static_cast<double>(face.width) * kCropMargin
    );
    const int margin_y = static_cast<int>(
        static_cast<double>(face.height) * kCropMargin
    );

    const int x1 = std::max(0, face.x - margin_x);
    const int y1 = std::max(0, face.y - margin_y);
    const int x2 = std::min(
        image.cols,
        face.x + face.width + margin_x
    );
    const int y2 = std::min(
        image.rows,
        face.y + face.height + margin_y
    );

    if (x2 <= x1 || y2 <= y1) {
        return {};
    }
    return image(cv::Rect(x1, y1, x2 - x1, y2 - y1));
}

cv::Mat normalized_face(const cv::Mat& image, const FPBiometricFace& face) {
    const cv::Mat crop = crop_face(image, face);
    if (crop.empty()) {
        return {};
    }

    cv::Mat gray;
    cv::cvtColor(crop, gray, cv::COLOR_BGR2GRAY);

    cv::Mat resized;
    cv::resize(
        gray,
        resized,
        cv::Size(96, 96),
        0.0,
        0.0,
        cv::INTER_LINEAR
    );

    cv::Mat normalized;
    resized.convertTo(normalized, CV_32F, 1.0 / 255.0);
    return normalized;
}

double robust_mean(const std::vector<double>& values) {
    if (values.empty()) {
        return 0.0;
    }
    return fp_bio_robust_mean(values.data(), values.size());
}

const double* data_or_null(const std::vector<double>& values) {
    return values.empty() ? nullptr : values.data();
}

struct FrameState {
    int frame_index = -1;
    int phase = FP_SECURE_PHASE_FAR;
    FPBiometricFace face{};
    FPBiometricQuality quality{};
    std::vector<float> embedding;
    cv::Mat normalized_face;
    FPBiometricCenter center{};
    double height_ratio = 0.0;
};

bool normalize_combined(
    const std::vector<FrameState*>& selected,
    float* output,
    size_t capacity,
    size_t* output_size
) {
    if (selected.empty() || output == nullptr || output_size == nullptr) {
        return false;
    }

    const size_t dimension = selected.front()->embedding.size();
    if (dimension == 0 || dimension > capacity) {
        return false;
    }

    std::vector<double> combined(dimension, 0.0);
    for (const FrameState* frame : selected) {
        if (frame == nullptr || frame->embedding.size() != dimension) {
            return false;
        }
        for (size_t index = 0; index < dimension; ++index) {
            combined[index] += static_cast<double>(frame->embedding[index]);
        }
    }

    const double divisor = static_cast<double>(selected.size());
    double norm_squared = 0.0;
    for (double& value : combined) {
        value /= divisor;
        norm_squared += value * value;
    }

    const double norm = std::sqrt(norm_squared);
    if (norm <= 1e-8) {
        return false;
    }

    for (size_t index = 0; index < dimension; ++index) {
        output[index] = static_cast<float>(combined[index] / norm);
    }
    *output_size = dimension;
    return true;
}

std::string lower_error(
    const char* operation,
    int code,
    const char* detail
) {
    std::ostringstream stream;
    stream << operation << " failed with code " << code;
    if (detail != nullptr && detail[0] != '\0') {
        stream << ": " << detail;
    }
    return stream.str();
}

}  // namespace

extern "C" {

int fp_secure_core_abi_version(void) {
    return FP_SECURE_CORE_ABI_VERSION;
}

const char* fp_secure_core_version(void) {
    return FP_SECURE_CORE_VERSION;
}

const char* fp_secure_core_last_error(void) {
    return g_last_error.c_str();
}

FPSecureCore* fp_secure_core_create(
    const char* yunet_model_path,
    const char* sface_model_path,
    const char* minifasnet_model_path
) {
    if (yunet_model_path == nullptr ||
        sface_model_path == nullptr ||
        minifasnet_model_path == nullptr) {
        set_last_error("all model paths are required");
        return nullptr;
    }

    try {
        auto core = std::make_unique<FPSecureCore>();
        core->vision = fp_bio_vision_create(
            yunet_model_path,
            sface_model_path
        );
        if (core->vision == nullptr) {
            set_last_error("could not create YuNet/SFace vision engine");
            return nullptr;
        }

        core->pad = fp_bio_pad_create(minifasnet_model_path);
        if (core->pad == nullptr) {
            const char* detail = fp_bio_pad_last_error();
            set_last_error(
                detail != nullptr && detail[0] != '\0'
                    ? detail
                    : "could not create MiniFASNet PAD engine"
            );
            fp_bio_vision_destroy(core->vision);
            core->vision = nullptr;
            return nullptr;
        }

        set_last_error("");
        return core.release();
    } catch (const std::exception& error) {
        set_last_error(error.what());
        return nullptr;
    } catch (...) {
        set_last_error("unknown Secure Core initialization error");
        return nullptr;
    }
}

void fp_secure_core_destroy(FPSecureCore* core) {
    if (core == nullptr) {
        return;
    }
    fp_bio_pad_destroy(core->pad);
    fp_bio_vision_destroy(core->vision);
    delete core;
}

int fp_secure_core_analyze_identity(
    FPSecureCore* core,
    const FPSecureCoreFrameInput* frames,
    size_t frame_count,
    FPSecureCoreIdentityResult* result
) {
    if (core == nullptr || frames == nullptr || result == nullptr) {
        set_last_error("invalid Secure Core identity arguments");
        return -1;
    }
    if (frame_count < 6 || frame_count > 12) {
        set_last_error("identity capture must contain between 6 and 12 frames");
        return -2;
    }

    std::lock_guard<std::mutex> lock(core->mutex);

    try {
        set_last_error("");
        *result = {};

        std::vector<FrameState> analyzed;
        analyzed.reserve(frame_count);

        std::vector<double> far_scales;
        std::vector<double> near_scales;
        std::vector<double> center_scores;
        std::vector<double> quality_scores;
        std::vector<double> passive_values;
        std::vector<double> sharpness_values;
        std::vector<double> brightness_values;
        std::vector<double> face_size_values;

        size_t far_count = 0;
        size_t near_count = 0;

        for (size_t index = 0; index < frame_count; ++index) {
            const FPSecureCoreFrameInput& input = frames[index];
            if (input.data == nullptr || input.size == 0) {
                continue;
            }
            if (input.phase != FP_SECURE_PHASE_FAR &&
                input.phase != FP_SECURE_PHASE_NEAR) {
                continue;
            }

            std::vector<float> embedding(
                FP_SECURE_CORE_EMBEDDING_CAPACITY
            );
            size_t embedding_size = 0;
            FPBiometricFace face{};
            FPBiometricQuality quality{};

            const int vision_code = fp_bio_vision_encode_jpeg(
                core->vision,
                input.data,
                input.size,
                embedding.data(),
                embedding.size(),
                &embedding_size,
                &face,
                &quality
            );
            if (vision_code == 1) {
                continue;
            }
            if (vision_code != 0) {
                set_last_error(
                    lower_error(
                        "vision",
                        vision_code,
                        fp_bio_vision_last_error()
                    )
                );
                return -10;
            }
            embedding.resize(embedding_size);

            const cv::Mat image = decode_jpeg(input.data, input.size);
            if (image.empty()) {
                set_last_error("could not decode identity JPEG");
                return -11;
            }

            FPBiometricPADResult pad_result{};
            const int pad_code = fp_bio_pad_predict_jpeg(
                core->pad,
                input.data,
                input.size,
                &face,
                &pad_result
            );
            if (pad_code == 0) {
                passive_values.push_back(pad_result.real_probability);
            }

            const cv::Mat normalized = normalized_face(image, face);
            if (normalized.empty()) {
                continue;
            }

            const double image_width =
                static_cast<double>(std::max(image.cols, 1));
            const double image_height =
                static_cast<double>(std::max(image.rows, 1));
            const double center_x =
                (static_cast<double>(face.x) +
                 static_cast<double>(face.width) / 2.0) /
                image_width;
            const double center_y =
                (static_cast<double>(face.y) +
                 static_cast<double>(face.height) / 2.0) /
                image_height;
            const double normalized_scale = std::sqrt(
                (
                    static_cast<double>(face.width) *
                    static_cast<double>(face.height)
                ) /
                std::max(image_width * image_height, 1.0)
            );
            const double height_ratio =
                static_cast<double>(face.height) / image_height;

            const double dx = center_x - 0.5;
            const double dy = center_y - 0.46;
            const double center_distance = std::sqrt(dx * dx + dy * dy);
            const double center_score =
                clamp01(1.0 - center_distance / 0.24);

            center_scores.push_back(center_score);
            quality_scores.push_back(quality.score);
            sharpness_values.push_back(quality.sharpness);
            brightness_values.push_back(quality.brightness);
            face_size_values.push_back(quality.face_size);

            if (input.phase == FP_SECURE_PHASE_FAR) {
                far_scales.push_back(height_ratio);
                ++far_count;
            } else {
                near_scales.push_back(height_ratio);
                ++near_count;
            }

            FrameState state;
            state.frame_index = static_cast<int>(index);
            state.phase = input.phase;
            state.face = face;
            state.quality = quality;
            state.embedding = std::move(embedding);
            state.normalized_face = normalized;
            state.center = {
                center_x,
                center_y,
                normalized_scale,
            };
            state.height_ratio = height_ratio;
            analyzed.push_back(std::move(state));
        }

        if (far_count < 3 || near_count < 3) {
            set_last_error(
                "identity capture requires at least three valid far and near frames"
            );
            return -20;
        }
        if (analyzed.empty()) {
            set_last_error("no face detected in identity capture");
            return -21;
        }

        std::vector<double> pixel_differences;
        if (analyzed.size() > 1) {
            pixel_differences.reserve(analyzed.size() - 1);
            for (size_t index = 1; index < analyzed.size(); ++index) {
                cv::Mat difference;
                cv::absdiff(
                    analyzed[index].normalized_face,
                    analyzed[index - 1].normalized_face,
                    difference
                );
                pixel_differences.push_back(cv::mean(difference)[0]);
            }
        }

        std::vector<FPBiometricCenter> centers;
        centers.reserve(analyzed.size());
        for (const auto& frame : analyzed) {
            centers.push_back(frame.center);
        }

        const double face_presence =
            static_cast<double>(analyzed.size()) /
            static_cast<double>(frame_count);
        const double quality_score = robust_mean(quality_scores);
        const double sharpness = robust_mean(sharpness_values);
        const double brightness = robust_mean(brightness_values);
        const double face_size = robust_mean(face_size_values);

        const double temporal_score = fp_bio_temporal_motion_score(
            data_or_null(pixel_differences),
            pixel_differences.size(),
            centers.data(),
            centers.size()
        );

        const double guided_score = fp_bio_guided_capture_score(
            data_or_null(far_scales),
            far_scales.size(),
            data_or_null(near_scales),
            near_scales.size(),
            data_or_null(center_scores),
            center_scores.size(),
            data_or_null(quality_scores),
            quality_scores.size()
        );

        const bool passive_available = !passive_values.empty();
        const double passive_score = passive_available
            ? robust_mean(passive_values)
            : 0.5;

        const double liveness_score = fp_bio_identity_liveness_score(
            passive_score,
            passive_available ? 1 : 0,
            guided_score,
            temporal_score,
            quality_score,
            face_presence
        );

        std::vector<FrameState*> near_candidates;
        for (auto& frame : analyzed) {
            if (frame.phase == FP_SECURE_PHASE_NEAR) {
                near_candidates.push_back(&frame);
            }
        }
        std::stable_sort(
            near_candidates.begin(),
            near_candidates.end(),
            [](const FrameState* left, const FrameState* right) {
                return left->quality.score > right->quality.score;
            }
        );

        if (near_candidates.size() < FP_SECURE_CORE_MAX_SELECTED_FRAMES) {
            set_last_error("not enough valid near embeddings");
            return -22;
        }
        near_candidates.resize(FP_SECURE_CORE_MAX_SELECTED_FRAMES);

        size_t combined_size = 0;
        if (!normalize_combined(
                near_candidates,
                result->embedding,
                FP_SECURE_CORE_EMBEDDING_CAPACITY,
                &combined_size
            )) {
            set_last_error("could not combine SFace embeddings");
            return -23;
        }

        result->liveness_score = liveness_score;
        result->passive_pad_score = passive_score;
        result->passive_pad_available = passive_available ? 1 : 0;
        result->temporal_motion_score = temporal_score;
        result->guided_capture_score = guided_score;
        result->quality_score = quality_score;
        result->face_presence = face_presence;
        result->sharpness = sharpness;
        result->brightness = brightness;
        result->face_size = face_size;
        result->detected_frames = static_cast<int32_t>(analyzed.size());
        result->processed_frames = static_cast<int32_t>(frame_count);
        result->best_frame_index =
            static_cast<int32_t>(near_candidates.front()->frame_index);
        result->embedding_size = combined_size;
        result->selected_embedding_count =
            FP_SECURE_CORE_MAX_SELECTED_FRAMES;

        for (size_t selected_index = 0;
             selected_index < FP_SECURE_CORE_MAX_SELECTED_FRAMES;
             ++selected_index) {
            const FrameState& frame = *near_candidates[selected_index];
            FPSecureCoreSelectedEmbedding& output =
                result->selected_embeddings[selected_index];

            output.frame_index = frame.frame_index;
            output.phase = frame.phase;
            output.quality = frame.quality.score;
            output.embedding_size = frame.embedding.size();

            if (output.embedding_size >
                FP_SECURE_CORE_EMBEDDING_CAPACITY) {
                set_last_error("selected SFace embedding exceeds ABI capacity");
                return -24;
            }
            std::copy(
                frame.embedding.begin(),
                frame.embedding.end(),
                output.embedding
            );
        }

        return 0;
    } catch (const cv::Exception& error) {
        set_last_error(error.what());
        return -30;
    } catch (const std::exception& error) {
        set_last_error(error.what());
        return -31;
    } catch (...) {
        set_last_error("unknown Secure Core identity error");
        return -32;
    }
}

int fp_secure_core_reference_jpeg(
    FPSecureCore* core,
    const uint8_t* jpeg_data,
    size_t jpeg_size,
    FPSecureCoreReferenceResult* result
) {
    if (core == nullptr || jpeg_data == nullptr ||
        jpeg_size == 0 || result == nullptr) {
        set_last_error("invalid Secure Core reference arguments");
        return -1;
    }

    std::lock_guard<std::mutex> lock(core->mutex);

    *result = {};
    size_t embedding_size = 0;
    size_t variant_count = 0;

    // The lower vision API returns reference variants tightly packed using
    // the real embedding dimension. The public Secure Core ABI stores each
    // variant in a fixed-capacity row, so copy through a packed buffer and
    // explicitly expand it to the ABI stride after dimensions are known.
    std::vector<float> packed_variants(
        FP_SECURE_CORE_EMBEDDING_CAPACITY *
        FP_SECURE_CORE_MAX_SELECTED_FRAMES,
        0.0F
    );

    const int code = fp_bio_vision_reference_jpeg(
        core->vision,
        jpeg_data,
        jpeg_size,
        result->embedding,
        FP_SECURE_CORE_EMBEDDING_CAPACITY,
        packed_variants.data(),
        packed_variants.size(),
        &embedding_size,
        &variant_count,
        &result->face,
        &result->quality
    );
    if (code != 0) {
        set_last_error(
            lower_error(
                "reference",
                code,
                fp_bio_vision_last_error()
            )
        );
        return code;
    }

    if (embedding_size == 0 ||
        embedding_size > FP_SECURE_CORE_EMBEDDING_CAPACITY ||
        variant_count == 0 ||
        variant_count > FP_SECURE_CORE_MAX_SELECTED_FRAMES) {
        set_last_error("reference embedding exceeds Secure Core ABI capacity");
        return -4;
    }

    for (size_t variant_index = 0;
         variant_index < variant_count;
         ++variant_index) {
        const size_t packed_offset = variant_index * embedding_size;
        std::copy_n(
            packed_variants.begin() + packed_offset,
            embedding_size,
            result->variants[variant_index]
        );

        double norm_squared = 0.0;
        for (size_t index = 0; index < embedding_size; ++index) {
            const double value = static_cast<double>(
                result->variants[variant_index][index]
            );
            if (!std::isfinite(value)) {
                set_last_error("reference variant contains non-finite values");
                return -5;
            }
            norm_squared += value * value;
        }
        if (norm_squared <= 1e-12) {
            set_last_error("reference variant has zero norm");
            return -5;
        }
    }

    result->embedding_size = embedding_size;
    result->variant_count = variant_count;
    set_last_error("");
    return 0;
}

int fp_secure_core_guide_jpeg(
    FPSecureCore* core,
    const uint8_t* jpeg_data,
    size_t jpeg_size,
    FPSecureCoreGuideResult* result
) {
    if (core == nullptr || jpeg_data == nullptr ||
        jpeg_size == 0 || result == nullptr) {
        set_last_error("invalid Secure Core guide arguments");
        return -1;
    }

    std::lock_guard<std::mutex> lock(core->mutex);

    try {
        *result = {};
        const cv::Mat image = decode_jpeg(jpeg_data, jpeg_size);
        if (image.empty()) {
            set_last_error("could not decode guide JPEG");
            return -2;
        }
        result->image_width = image.cols;
        result->image_height = image.rows;

        FPBiometricFace face{};
        FPBiometricQuality quality{};
        const int code = fp_bio_vision_analyze_jpeg(
            core->vision,
            jpeg_data,
            jpeg_size,
            &face,
            &quality
        );
        if (code == 1) {
            set_last_error("");
            return 0;
        }
        if (code != 0) {
            set_last_error(
                lower_error(
                    "guide",
                    code,
                    fp_bio_vision_last_error()
                )
            );
            return code;
        }

        const double width =
            static_cast<double>(std::max(image.cols, 1));
        const double height =
            static_cast<double>(std::max(image.rows, 1));

        result->face_detected = 1;
        result->confidence = face.confidence;
        result->center_x =
            (static_cast<double>(face.x) +
             static_cast<double>(face.width) / 2.0) /
            width;
        result->center_y =
            (static_cast<double>(face.y) +
             static_cast<double>(face.height) / 2.0) /
            height;
        result->width_ratio =
            static_cast<double>(face.width) / width;
        result->height_ratio =
            static_cast<double>(face.height) / height;
        result->quality = quality;

        const double right_eye_x = static_cast<double>(face.raw[4]);
        const double right_eye_y = static_cast<double>(face.raw[5]);
        const double left_eye_x = static_cast<double>(face.raw[6]);
        const double left_eye_y = static_cast<double>(face.raw[7]);
        constexpr double radians_to_degrees =
            180.0 / 3.14159265358979323846;
        result->roll_degrees = std::atan2(
            left_eye_y - right_eye_y,
            left_eye_x - right_eye_x
        ) * radians_to_degrees;

        set_last_error("");
        return 0;
    } catch (const cv::Exception& error) {
        set_last_error(error.what());
        return -3;
    } catch (const std::exception& error) {
        set_last_error(error.what());
        return -3;
    } catch (...) {
        set_last_error("unknown Secure Core guide error");
        return -3;
    }
}

}  // extern "C"
