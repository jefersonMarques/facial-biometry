#include "faceproof_biometric_vision.h"

#include <opencv2/core.hpp>
#include <opencv2/imgcodecs.hpp>
#include <opencv2/imgproc.hpp>

#include <algorithm>
#include <cmath>
#include <cstdint>
#include <fstream>
#include <iomanip>
#include <iostream>
#include <iterator>
#include <limits>
#include <string>
#include <utility>
#include <vector>

namespace {

constexpr size_t kMaxInputBytes = 8U << 20;
constexpr size_t kEmbeddingCapacity = 512;
constexpr size_t kVariantCapacity = 512 * 3;
constexpr int kMaxOriginalDimension = 4096;
constexpr double kMaximumLongSide = 1280.0;
constexpr double kMinimumShortSide = 480.0;

struct ImageReference {
    bool valid = false;
    int code = 0;
    size_t dimension = 0;
    FPBiometricFace face{};
    FPBiometricQuality quality{};
    std::vector<double> combined;
    std::vector<std::vector<double>> variants;
};

struct PairScore {
    bool valid = false;
    double combined = 0.0;
    double robust = 0.0;
};

std::vector<uint8_t> read_image(const std::string& path) {
    std::ifstream file(path, std::ios::binary | std::ios::ate);
    if (!file || file.tellg() <= 0 ||
        static_cast<uint64_t>(file.tellg()) > kMaxInputBytes) {
        return {};
    }
    file.seekg(0);
    return std::vector<uint8_t>(
        std::istreambuf_iterator<char>(file),
        std::istreambuf_iterator<char>()
    );
}

cv::Mat decode_image(const std::vector<uint8_t>& bytes) {
    if (bytes.empty()) {
        return {};
    }
    const cv::Mat encoded(
        1, static_cast<int>(bytes.size()), CV_8UC1,
        const_cast<uint8_t*>(bytes.data())
    );
    const cv::Mat image = cv::imdecode(encoded, cv::IMREAD_COLOR);
    if (image.empty() || image.cols > kMaxOriginalDimension ||
        image.rows > kMaxOriginalDimension) {
        return {};
    }
    return image;
}

cv::Size production_target_size(const cv::Size& size) {
    if (size.width <= 0 || size.height <= 0) {
        return {};
    }
    const int short_side = std::min(size.width, size.height);
    const int long_side = std::max(size.width, size.height);
    if (short_side >= kMinimumShortSide || long_side >= kMaximumLongSide) {
        return size;
    }
    double scale = kMinimumShortSide / static_cast<double>(short_side);
    if (static_cast<double>(long_side) * scale > kMaximumLongSide) {
        scale = kMaximumLongSide / static_cast<double>(long_side);
    }
    return {
        std::max(1, static_cast<int>(std::nearbyint(size.width * scale))),
        std::max(1, static_cast<int>(std::nearbyint(size.height * scale)))
    };
}

ImageReference extract_reference(
    FPBiometricVisionEngine* engine,
    const std::vector<uint8_t>& encoded
) {
    ImageReference result;
    std::vector<float> combined(kEmbeddingCapacity);
    std::vector<float> variants(kVariantCapacity);
    size_t dimension = 0;
    size_t count = 0;
    result.code = fp_bio_vision_reference_jpeg(
        engine, encoded.data(), encoded.size(),
        combined.data(), combined.size(),
        variants.data(), variants.size(),
        &dimension, &count, &result.face, &result.quality
    );
    if (result.code != 0 || dimension == 0 ||
        dimension > kEmbeddingCapacity || count == 0 ||
        count > kVariantCapacity / dimension) {
        return result;
    }
    result.dimension = dimension;
    result.combined.assign(combined.begin(), combined.begin() + dimension);
    for (size_t n = 0; n < count; ++n) {
        std::vector<double> row;
        for (size_t i = 0; i < dimension; ++i) {
            row.push_back(variants[n * dimension + i]);
        }
        result.variants.push_back(std::move(row));
    }
    result.valid = true;
    return result;
}

double cosine(const std::vector<double>& left, const std::vector<double>& right) {
    if (left.size() != right.size() || left.empty()) {
        return std::numeric_limits<double>::quiet_NaN();
    }
    double dot = 0.0, norm_left = 0.0, norm_right = 0.0;
    for (size_t i = 0; i < left.size(); ++i) {
        dot += left[i] * right[i];
        norm_left += left[i] * left[i];
        norm_right += right[i] * right[i];
    }
    if (norm_left <= 0 || norm_right <= 0) {
        return std::numeric_limits<double>::quiet_NaN();
    }
    return dot / std::sqrt(norm_left * norm_right);
}

PairScore compare(const ImageReference& reference, const ImageReference& target) {
    PairScore result;
    if (!reference.valid || !target.valid ||
        reference.dimension != target.dimension) {
        return result;
    }
    result.combined = cosine(reference.combined, target.combined);
    if (!std::isfinite(result.combined)) {
        return result;
    }
    double sum = 0.0;
    for (const auto& target_embedding : target.variants) {
        std::vector<double> scores;
        for (const auto& reference_embedding : reference.variants) {
            const double score = cosine(reference_embedding, target_embedding);
            if (std::isfinite(score)) {
                scores.push_back(score);
            }
        }
        if (scores.empty()) {
            return result;
        }
        std::sort(scores.begin(), scores.end());
        const size_t mid = scores.size() / 2;
        sum += scores.size() % 2 != 0
            ? scores[mid]
            : (scores[mid - 1] + scores[mid]) / 2.0;
    }
    if (target.variants.empty()) {
        return result;
    }
    result.robust = sum / static_cast<double>(target.variants.size());
    result.valid = std::isfinite(result.robust);
    return result;
}

void print_pair(const char* key, const PairScore& pair) {
    std::cout << "\"" << key << "\":{"
              << "\"status\":\"" << (pair.valid ? "compared" : "unavailable") << "\"";
    if (pair.valid) {
        std::cout << ",\"combinedCosine\":" << pair.combined
                  << ",\"robustCosine\":" << pair.robust;
    }
    std::cout << "}";
}

void print_quality(const ImageReference& reference) {
    std::cout << "{\"score\":" << reference.quality.score
              << ",\"sharpness\":" << reference.quality.sharpness
              << ",\"brightness\":" << reference.quality.brightness
              << ",\"faceSize\":" << reference.quality.face_size
              << "}";
}

void print_variant(
    const char* name,
    const ImageReference& pdf,
    const ImageReference& vio,
    const ImageReference& live,
    const cv::Size& source_size,
    const cv::Size& input_size,
    bool baseline
) {
    std::cout << "{\"method\":\"" << name << "\""
              << ",\"status\":\"" << (vio.valid ? "analyzed" : "analysis_failed") << "\""
              << ",\"inputWidth\":" << input_size.width
              << ",\"inputHeight\":" << input_size.height
              << ",\"sourceWidth\":" << source_size.width
              << ",\"sourceHeight\":" << source_size.height
              << ",\"scaleX\":" << static_cast<double>(input_size.width) / source_size.width
              << ",\"scaleY\":" << static_cast<double>(input_size.height) / source_size.height
              << ",\"referenceSuccess\":" << (vio.valid ? "true" : "false");
    if (!vio.valid) {
        std::cout << ",\"referenceCode\":" << vio.code;
    } else {
        std::cout << ",\"faceWidthProcessed\":" << vio.face.width
                  << ",\"faceHeightProcessed\":" << vio.face.height
                  << ",\"faceWidthEstimatedSource\":"
                  << vio.face.width * static_cast<double>(source_size.width) /
                         static_cast<double>(input_size.width)
                  << ",\"faceHeightEstimatedSource\":"
                  << vio.face.height * static_cast<double>(source_size.height) /
                         static_cast<double>(input_size.height)
                  << ",\"quality\":";
        print_quality(vio);
    }
    std::cout << ",\"baseline\":" << (baseline ? "true" : "false") << ",";
    print_pair("pdfVio", compare(pdf, vio));
    std::cout << ",";
    print_pair("vioLive", compare(vio, live));
    std::cout << ",";
    print_pair("pdfLive", compare(pdf, live));
    std::cout << "}";
}

}  // namespace

int main(int argc, char** argv) {
    if (argc != 6) {
        std::cerr << "usage: faceproof_low_resolution_benchmark "
                  << "<pdf_portrait> <vio_portrait> <live_reference_photo> "
                  << "<yunet.onnx> <sface.onnx>\n";
        return 2;
    }
    const auto pdf_data = read_image(argv[1]);
    const auto vio_data = read_image(argv[2]);
    const auto live_data = read_image(argv[3]);
    const cv::Mat pdf_image = decode_image(pdf_data);
    const cv::Mat vio_image = decode_image(vio_data);
    const cv::Mat live_image = decode_image(live_data);
    if (pdf_image.empty() || vio_image.empty() || live_image.empty()) {
        std::cerr << "invalid or oversized image input\n";
        return 3;
    }

    FPBiometricVisionEngine* engine = fp_bio_vision_create(argv[4], argv[5]);
    if (engine == nullptr) {
        std::cerr << "cannot initialize vision models\n";
        return 4;
    }
    const auto pdf = extract_reference(engine, pdf_data);
    const auto live = extract_reference(engine, live_data);
    if (!pdf.valid || !live.valid) {
        fp_bio_vision_destroy(engine);
        std::cerr << "reference face not found in PDF or live proxy image\n";
        return 5;
    }

    const cv::Size target_size = production_target_size(vio_image.size());
    const auto baseline = extract_reference(engine, vio_data);

    std::cout << std::setprecision(17)
              << "{\"schemaVersion\":1,"
              << "\"mode\":\"offline_low_resolution_diagnostic\","
              << "\"liveIsSingleFrameProxy\":true,"
              << "\"vioOriginalWidth\":" << vio_image.cols << ","
              << "\"vioOriginalHeight\":" << vio_image.rows << ","
              << "\"productionTargetWidth\":" << target_size.width << ","
              << "\"productionTargetHeight\":" << target_size.height << ","
              << "\"pdfQuality\":";
    print_quality(pdf);
    std::cout << ",\"liveQuality\":";
    print_quality(live);
    std::cout << ",\"variants\":[";
    print_variant("production_cubic", pdf, baseline, live,
                  vio_image.size(), target_size, true);

    if (target_size != vio_image.size()) {
        const std::pair<const char*, int> variants[] = {
            {"nearest", cv::INTER_NEAREST},
            {"linear", cv::INTER_LINEAR},
            {"lanczos4", cv::INTER_LANCZOS4}
        };
        for (const auto& variant : variants) {
            cv::Mat resized;
            cv::resize(vio_image, resized, target_size, 0, 0, variant.second);
            std::vector<uint8_t> encoded;
            if (!cv::imencode(".png", resized, encoded)) {
                continue;
            }
            const auto reference = extract_reference(engine, encoded);
            std::cout << ",";
            print_variant(variant.first, pdf, reference, live,
                          vio_image.size(), target_size, false);
        }
    }
    std::cout << "]}\n";
    fp_bio_vision_destroy(engine);
    return 0;
}
