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

}  // namespace

int main(int argc, char** argv) {
    if (argc != 4) {
        std::cerr << "usage: faceproof_biometric_vision_cli <image.jpg> <yunet.onnx> <sface.onnx>\n";
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

    std::vector<float> embedding(512);
    size_t embedding_size = 0;
    FPBiometricFace face{};
    FPBiometricQuality quality{};

    const int code = fp_bio_vision_encode_jpeg(
        engine,
        image.data(),
        image.size(),
        embedding.data(),
        embedding.size(),
        &embedding_size,
        &face,
        &quality
    );
    fp_bio_vision_destroy(engine);

    if (code != 0) {
        std::cerr << "vision analysis failed: " << code << "\n";
        return 5;
    }

    std::cout << std::setprecision(17)
              << "{"
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
              << "\"brightnessValue\":" << quality.brightness_value << ","
              << "\"faceSize\":" << quality.face_size << ","
              << "\"score\":" << quality.score
              << "},"
              << "\"embedding\":[";

    for (size_t index = 0; index < embedding_size; ++index) {
        if (index > 0) {
            std::cout << ",";
        }
        std::cout << embedding[index];
    }

    std::cout << "]}\n";
    return 0;
}
