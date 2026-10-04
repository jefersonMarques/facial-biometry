#include "faceproof_biometric_vision.h"

#include <fstream>
#include <iomanip>
#include <iostream>
#include <iterator>
#include <string>
#include <vector>

namespace {

std::vector<uint8_t> read_file(const std::string& path) {
    std::ifstream input(path, std::ios::binary);
    if (!input) {
        return {};
    }
    return std::vector<uint8_t>(
        std::istreambuf_iterator<char>(input),
        std::istreambuf_iterator<char>()
    );
}

void print_vector(const float* values, size_t count) {
    std::cout << "[";
    for (size_t index = 0; index < count; ++index) {
        if (index > 0) {
            std::cout << ",";
        }
        std::cout << values[index];
    }
    std::cout << "]";
}

}  // namespace

int main(int argc, char** argv) {
    if (argc != 4) {
        std::cerr
            << "usage: faceproof_biometric_reference_cli "
            << "<image> <yunet.onnx> <sface.onnx>\n";
        return 2;
    }

    const auto image = read_file(argv[1]);
    if (image.empty()) {
        std::cerr << "image could not be read\n";
        return 3;
    }

    FPBiometricVisionEngine* engine =
        fp_bio_vision_create(argv[2], argv[3]);
    if (engine == nullptr) {
        std::cerr << "vision engine could not be created\n";
        return 4;
    }

    std::vector<float> combined(512);
    std::vector<float> variants(512 * 3);
    size_t embedding_size = 0;
    size_t variant_count = 0;
    FPBiometricFace face{};
    FPBiometricQuality quality{};

    const int code = fp_bio_vision_reference_jpeg(
        engine,
        image.data(),
        image.size(),
        combined.data(),
        combined.size(),
        variants.data(),
        variants.size(),
        &embedding_size,
        &variant_count,
        &face,
        &quality
    );
    fp_bio_vision_destroy(engine);

    if (code != 0) {
        std::cerr
            << "reference analysis failed: "
            << code
            << " embedding_size="
            << embedding_size
            << " variant_count="
            << variant_count
            << " error="
            << fp_bio_vision_last_error()
            << "\n";
        return 5;
    }

    std::cout << std::setprecision(17)
              << "{"
              << "\"embedding\":";
    print_vector(combined.data(), embedding_size);

    std::cout << ",\"embeddings\":[";
    for (size_t variant = 0; variant < variant_count; ++variant) {
        if (variant > 0) {
            std::cout << ",";
        }
        print_vector(
            variants.data() + variant * embedding_size,
            embedding_size
        );
    }
    std::cout << "],"
              << "\"face\":{"
              << "\"x\":" << face.x << ","
              << "\"y\":" << face.y << ","
              << "\"width\":" << face.width << ","
              << "\"height\":" << face.height << ","
              << "\"confidence\":" << face.confidence
              << "},"
              << "\"quality\":{"
              << "\"sharpness\":" << quality.sharpness << ","
              << "\"brightness\":" << quality.brightness << ","
              << "\"faceSize\":" << quality.face_size << ","
              << "\"score\":" << quality.score
              << "}"
              << "}\n";

    return 0;
}
