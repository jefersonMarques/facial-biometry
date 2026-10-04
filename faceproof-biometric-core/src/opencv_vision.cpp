#include "faceproof_biometric_vision.h"

#include <opencv2/core.hpp>
#include <opencv2/imgcodecs.hpp>
#include <opencv2/imgproc.hpp>
#include <opencv2/objdetect/face.hpp>

#include <algorithm>
#include <cmath>
#include <cstring>
#include <memory>
#include <vector>

namespace {

constexpr double kYuNetScoreThreshold = 0.82;
constexpr double kYuNetNMSThreshold = 0.3;
constexpr int kYuNetTopK = 5000;
constexpr double kCropMargin = 0.08;

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

int round_like_python(float value) {
    return static_cast<int>(std::nearbyint(static_cast<double>(value)));
}

cv::Rect clamp_bbox(const cv::Mat& image, const cv::Mat& row) {
    int x = std::max(0, round_like_python(row.at<float>(0, 0)));
    int y = std::max(0, round_like_python(row.at<float>(0, 1)));
    int width = std::max(1, round_like_python(row.at<float>(0, 2)));
    int height = std::max(1, round_like_python(row.at<float>(0, 3)));

    if (x >= image.cols) {
        x = std::max(0, image.cols - 1);
    }
    if (y >= image.rows) {
        y = std::max(0, image.rows - 1);
    }

    width = std::max(1, std::min(image.cols - x, width));
    height = std::max(1, std::min(image.rows - y, height));
    return cv::Rect(x, y, width, height);
}

cv::Mat crop_face(const cv::Mat& image, const cv::Rect& bbox) {
    const int margin_x = static_cast<int>(
        static_cast<double>(bbox.width) * kCropMargin
    );
    const int margin_y = static_cast<int>(
        static_cast<double>(bbox.height) * kCropMargin
    );

    const int x1 = std::max(0, bbox.x - margin_x);
    const int y1 = std::max(0, bbox.y - margin_y);
    const int x2 = std::min(image.cols, bbox.x + bbox.width + margin_x);
    const int y2 = std::min(image.rows, bbox.y + bbox.height + margin_y);

    if (x2 <= x1 || y2 <= y1) {
        return {};
    }
    return image(cv::Rect(x1, y1, x2 - x1, y2 - y1));
}

double sharpness_score(const cv::Mat& image) {
    cv::Mat gray;
    cv::cvtColor(image, gray, cv::COLOR_BGR2GRAY);

    cv::Mat laplacian;
    cv::Laplacian(gray, laplacian, CV_64F);

    cv::Scalar mean_value;
    cv::Scalar stddev;
    cv::meanStdDev(laplacian, mean_value, stddev);
    const double variance = stddev[0] * stddev[0];
    return clamp01((variance - 30.0) / 220.0);
}

std::pair<double, double> brightness_score(const cv::Mat& image) {
    cv::Mat gray;
    cv::cvtColor(image, gray, cv::COLOR_BGR2GRAY);
    const double normalized = cv::mean(gray)[0] / 255.0;
    const double score =
        1.0 - std::min(std::abs(normalized - 0.52) / 0.52, 1.0);
    return {clamp01(score), normalized};
}

double face_size_score(const cv::Mat& image, const cv::Rect& bbox) {
    const double image_area = std::max(
        static_cast<double>(image.cols) * static_cast<double>(image.rows),
        1.0
    );
    const double area_ratio =
        static_cast<double>(bbox.width) * static_cast<double>(bbox.height) /
        image_area;

    if (area_ratio < 0.07) {
        return clamp01(area_ratio / 0.07);
    }
    if (area_ratio > 0.62) {
        return clamp01(1.0 - ((area_ratio - 0.62) / 0.30));
    }
    return 1.0;
}

void fill_quality(
    const cv::Mat& image,
    const cv::Rect& bbox,
    FPBiometricQuality* output
) {
    if (output == nullptr) {
        return;
    }

    const cv::Mat face_crop = crop_face(image, bbox);
    if (face_crop.empty()) {
        *output = {};
        return;
    }

    const double sharpness = sharpness_score(face_crop);
    const auto brightness = brightness_score(face_crop);
    const double size_score = face_size_score(image, bbox);
    const double quality = clamp01(
        0.45 * sharpness +
        0.30 * brightness.first +
        0.25 * size_score
    );

    output->sharpness = sharpness;
    output->brightness = brightness.first;
    output->brightness_value = brightness.second;
    output->face_size = size_score;
    output->score = quality;
}

}  // namespace

struct FPBiometricVisionEngine {
    cv::Ptr<cv::FaceDetectorYN> detector;
    cv::Ptr<cv::FaceRecognizerSF> recognizer;
};

namespace {

bool detect_primary(
    FPBiometricVisionEngine* engine,
    const cv::Mat& image,
    cv::Mat* raw_face,
    cv::Rect* bbox,
    double* confidence
) {
    if (engine == nullptr || image.empty()) {
        return false;
    }

    engine->detector->setInputSize(image.size());

    cv::Mat faces;
    engine->detector->detect(image, faces);
    if (faces.empty()) {
        return false;
    }

    int best_index = 0;
    double best_rank = -1.0;
    for (int row = 0; row < faces.rows; ++row) {
        const float width = faces.at<float>(row, 2);
        const float height = faces.at<float>(row, 3);
        const float score = faces.at<float>(row, faces.cols - 1);
        const double rank =
            static_cast<double>(width) *
            static_cast<double>(height) *
            std::max(static_cast<double>(score), 0.01);
        if (rank > best_rank) {
            best_rank = rank;
            best_index = row;
        }
    }

    const cv::Mat selected = faces.row(best_index).clone();
    if (raw_face != nullptr) {
        *raw_face = selected;
    }
    if (bbox != nullptr) {
        *bbox = clamp_bbox(image, selected);
    }
    if (confidence != nullptr) {
        *confidence = static_cast<double>(
            selected.at<float>(0, selected.cols - 1)
        );
    }
    return true;
}

void fill_face(
    const cv::Mat& raw,
    const cv::Rect& bbox,
    double confidence,
    FPBiometricFace* output
) {
    if (output == nullptr) {
        return;
    }

    *output = {};
    output->detected = 1;
    output->x = bbox.x;
    output->y = bbox.y;
    output->width = bbox.width;
    output->height = bbox.height;
    output->confidence = confidence;

    const int count = std::min(raw.cols, 15);
    for (int index = 0; index < count; ++index) {
        output->raw[index] = raw.at<float>(0, index);
    }
}

}  // namespace

extern "C" {

FPBiometricVisionEngine* fp_bio_vision_create(
    const char* yunet_model_path,
    const char* sface_model_path
) {
    if (yunet_model_path == nullptr || sface_model_path == nullptr) {
        return nullptr;
    }

    try {
        auto engine = std::make_unique<FPBiometricVisionEngine>();
        engine->detector = cv::FaceDetectorYN::create(
            yunet_model_path,
            "",
            cv::Size(320, 320),
            static_cast<float>(kYuNetScoreThreshold),
            static_cast<float>(kYuNetNMSThreshold),
            kYuNetTopK
        );
        engine->recognizer = cv::FaceRecognizerSF::create(
            sface_model_path,
            ""
        );
        if (engine->detector.empty() || engine->recognizer.empty()) {
            return nullptr;
        }
        return engine.release();
    } catch (...) {
        return nullptr;
    }
}

void fp_bio_vision_destroy(FPBiometricVisionEngine* engine) {
    delete engine;
}

int fp_bio_vision_analyze_jpeg(
    FPBiometricVisionEngine* engine,
    const uint8_t* jpeg_data,
    size_t jpeg_size,
    FPBiometricFace* face,
    FPBiometricQuality* quality
) {
    if (engine == nullptr || jpeg_data == nullptr || jpeg_size == 0 ||
        face == nullptr || quality == nullptr) {
        return -1;
    }

    try {
        const cv::Mat image = decode_jpeg(jpeg_data, jpeg_size);
        if (image.empty()) {
            return -2;
        }

        cv::Mat raw;
        cv::Rect bbox;
        double confidence = 0.0;
        if (!detect_primary(engine, image, &raw, &bbox, &confidence)) {
            *face = {};
            *quality = {};
            return 1;
        }

        fill_face(raw, bbox, confidence, face);
        fill_quality(image, bbox, quality);
        return 0;
    } catch (...) {
        return -3;
    }
}

int fp_bio_vision_encode_jpeg(
    FPBiometricVisionEngine* engine,
    const uint8_t* jpeg_data,
    size_t jpeg_size,
    float* embedding,
    size_t embedding_capacity,
    size_t* embedding_size,
    FPBiometricFace* face,
    FPBiometricQuality* quality
) {
    if (engine == nullptr || jpeg_data == nullptr || jpeg_size == 0 ||
        embedding == nullptr || embedding_size == nullptr ||
        face == nullptr || quality == nullptr) {
        return -1;
    }

    try {
        const cv::Mat image = decode_jpeg(jpeg_data, jpeg_size);
        if (image.empty()) {
            return -2;
        }

        cv::Mat raw;
        cv::Rect bbox;
        double confidence = 0.0;
        if (!detect_primary(engine, image, &raw, &bbox, &confidence)) {
            *face = {};
            *quality = {};
            *embedding_size = 0;
            return 1;
        }

        cv::Mat aligned;
        engine->recognizer->alignCrop(image, raw, aligned);

        cv::Mat feature;
        engine->recognizer->feature(aligned, feature);
        cv::Mat flattened = feature.reshape(1, 1);
        flattened.convertTo(flattened, CV_32F);

        const size_t count = flattened.total();
        if (embedding_capacity < count) {
            *embedding_size = count;
            return -4;
        }

        const double norm = cv::norm(flattened, cv::NORM_L2);
        if (norm <= 1e-8) {
            return -5;
        }

        for (size_t index = 0; index < count; ++index) {
            embedding[index] =
                flattened.at<float>(0, static_cast<int>(index)) /
                static_cast<float>(norm);
        }
        *embedding_size = count;

        fill_face(raw, bbox, confidence, face);
        fill_quality(image, bbox, quality);
        return 0;
    } catch (...) {
        return -3;
    }
}

}  // extern "C"
