#include "faceproof_biometric_pad.h"
#include "faceproof_biometric_vision.h"

#include <cstdio>
#include <fstream>
#include <iomanip>
#include <iostream>
#include <iterator>
#include <string>
#include <vector>

#ifdef _WIN32
#include <fcntl.h>
#include <io.h>
#endif

namespace {

std::vector<uint8_t> read_file(const std::string& path) {
    if (path == "-") {
#ifdef _WIN32
        _setmode(_fileno(stdin), _O_BINARY);
#endif
        return std::vector<uint8_t>(
            std::istreambuf_iterator<char>(std::cin),
            std::istreambuf_iterator<char>()
        );
    }

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
    if (argc != 5) {
        std::cerr
            << "usage: faceproof_biometric_pad_cli "
            << "<image> <yunet.onnx> <sface.onnx> <minifas.onnx>\n";
        return 2;
    }

    const auto image = read_file(argv[1]);
    if (image.empty()) {
        std::cerr << "image could not be read\n";
        return 3;
    }

    FPBiometricVisionEngine* vision =
        fp_bio_vision_create(argv[2], argv[3]);
    if (vision == nullptr) {
        std::cerr
            << "vision engine could not be created: "
            << fp_bio_vision_last_error()
            << "\n";
        return 4;
    }

    FPBiometricFace face{};
    FPBiometricQuality quality{};
    const int vision_code = fp_bio_vision_analyze_jpeg(
        vision,
        image.data(),
        image.size(),
        &face,
        &quality
    );
    fp_bio_vision_destroy(vision);

    if (vision_code != 0) {
        std::cerr
            << "face detection failed: "
            << vision_code
            << " error="
            << fp_bio_vision_last_error()
            << "\n";
        return 5;
    }

    FPBiometricPAD* pad = fp_bio_pad_create(argv[4]);
    if (pad == nullptr) {
        std::cerr
            << "PAD engine could not be created: "
            << fp_bio_pad_last_error()
            << "\n";
        return 6;
    }

    FPBiometricPADResult result{};
    const int pad_code = fp_bio_pad_predict_jpeg(
        pad,
        image.data(),
        image.size(),
        &face,
        &result
    );
    fp_bio_pad_destroy(pad);

    if (pad_code != 0) {
        std::cerr
            << "PAD inference failed: "
            << pad_code
            << " error="
            << fp_bio_pad_last_error()
            << "\n";
        return 7;
    }

    std::cout << std::setprecision(17)
              << "{"
              << "\"realProbability\":"
              << result.real_probability
              << ",\"inputWidth\":"
              << result.input_width
              << ",\"inputHeight\":"
              << result.input_height
              << ",\"classCount\":"
              << result.class_count
              << ",\"face\":{"
              << "\"x\":" << face.x << ","
              << "\"y\":" << face.y << ","
              << "\"width\":" << face.width << ","
              << "\"height\":" << face.height
              << "}"
              << "}\n";

    return 0;
}
