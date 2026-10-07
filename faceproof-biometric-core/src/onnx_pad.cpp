#include "faceproof_biometric_pad.h"

#include <onnxruntime_cxx_api.h>

#include <opencv2/core.hpp>
#include <opencv2/imgcodecs.hpp>
#include <opencv2/imgproc.hpp>

#include <algorithm>
#include <array>
#include <cmath>
#include <memory>
#include <stdexcept>
#include <string>
#include <vector>

namespace {

thread_local std::string g_pad_last_error;

void set_last_error(const std::string& value) {
    g_pad_last_error = value;
}

cv::Mat decode_image(const uint8_t* data, size_t size) {
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

cv::Mat crop_scaled(
    const cv::Mat& image,
    const FPBiometricFace& face,
    double maximum_scale
) {
    const int source_height = image.rows;
    const int source_width = image.cols;
    const int x = face.x;
    const int y = face.y;
    const int width = std::max(face.width, 1);
    const int height = std::max(face.height, 1);

    const double scale = std::min({
        static_cast<double>(source_height - 1) /
            static_cast<double>(height),
        static_cast<double>(source_width - 1) /
            static_cast<double>(width),
        maximum_scale,
    });

    const double new_width = static_cast<double>(width) * scale;
    const double new_height = static_cast<double>(height) * scale;
    const double center_x =
        static_cast<double>(x) + static_cast<double>(width) / 2.0;
    const double center_y =
        static_cast<double>(y) + static_cast<double>(height) / 2.0;

    const int x1 = std::max(
        0,
        static_cast<int>(center_x - new_width / 2.0)
    );
    const int y1 = std::max(
        0,
        static_cast<int>(center_y - new_height / 2.0)
    );
    const int x2 = std::min(
        source_width - 1,
        static_cast<int>(center_x + new_width / 2.0)
    );
    const int y2 = std::min(
        source_height - 1,
        static_cast<int>(center_y + new_height / 2.0)
    );

    if (x2 < x1 || y2 < y1) {
        return {};
    }

    return image(
        cv::Rect(
            x1,
            y1,
            x2 - x1 + 1,
            y2 - y1 + 1
        )
    );
}

}  // namespace

struct FPBiometricPAD {
    Ort::Env env;
    Ort::SessionOptions options;
    std::unique_ptr<Ort::Session> session;
    std::string input_name;
    std::string output_name;
    int input_height = 0;
    int input_width = 0;

    explicit FPBiometricPAD(const char* model_path)
        : env(ORT_LOGGING_LEVEL_WARNING, "faceproof-pad") {
        options.SetGraphOptimizationLevel(
            GraphOptimizationLevel::ORT_ENABLE_ALL
        );
        options.SetIntraOpNumThreads(1);
        session = std::make_unique<Ort::Session>(
            env,
            model_path,
            options
        );

        Ort::AllocatorWithDefaultOptions allocator;
        const auto input_name_value =
            session->GetInputNameAllocated(0, allocator);
        const auto output_name_value =
            session->GetOutputNameAllocated(0, allocator);
        input_name = input_name_value.get();
        output_name = output_name_value.get();

        const auto input_shape =
            session
                ->GetInputTypeInfo(0)
                .GetTensorTypeAndShapeInfo()
                .GetShape();

        if (input_shape.size() != 4 ||
            input_shape[2] <= 0 ||
            input_shape[3] <= 0) {
            throw std::runtime_error(
                "MiniFASNet input must be fixed NCHW"
            );
        }

        input_height = static_cast<int>(input_shape[2]);
        input_width = static_cast<int>(input_shape[3]);
    }
};

extern "C" {

FPBiometricPAD* fp_bio_pad_create(const char* model_path) {
    if (model_path == nullptr) {
        return nullptr;
    }

    try {
        set_last_error("");
        return new FPBiometricPAD(model_path);
    } catch (const Ort::Exception& error) {
        set_last_error(error.what());
        return nullptr;
    } catch (const std::exception& error) {
        set_last_error(error.what());
        return nullptr;
    } catch (...) {
        set_last_error("unknown PAD initialization error");
        return nullptr;
    }
}

void fp_bio_pad_destroy(FPBiometricPAD* pad) {
    delete pad;
}

const char* fp_bio_pad_last_error(void) {
    return g_pad_last_error.c_str();
}

int fp_bio_pad_predict_jpeg(
    FPBiometricPAD* pad,
    const uint8_t* image_data,
    size_t image_size,
    const FPBiometricFace* face,
    FPBiometricPADResult* result
) {
    if (pad == nullptr ||
        image_data == nullptr ||
        image_size == 0 ||
        face == nullptr ||
        result == nullptr ||
        face->detected == 0) {
        return -1;
    }

    try {
        set_last_error("");

        const cv::Mat image = decode_image(image_data, image_size);
        if (image.empty()) {
            return -2;
        }

        const cv::Mat crop = crop_scaled(image, *face, 2.7);
        if (crop.empty()) {
            return -3;
        }

        cv::Mat resized;
        cv::resize(
            crop,
            resized,
            cv::Size(pad->input_width, pad->input_height)
        );

        const size_t plane_size =
            static_cast<size_t>(pad->input_width) *
            static_cast<size_t>(pad->input_height);
        std::vector<float> tensor(plane_size * 3);

        for (int y = 0; y < pad->input_height; ++y) {
            for (int x = 0; x < pad->input_width; ++x) {
                const cv::Vec3b pixel =
                    resized.at<cv::Vec3b>(y, x);
                const size_t offset =
                    static_cast<size_t>(y) *
                    static_cast<size_t>(pad->input_width) +
                    static_cast<size_t>(x);

                tensor[offset] =
                    static_cast<float>(pixel[0]);
                tensor[plane_size + offset] =
                    static_cast<float>(pixel[1]);
                tensor[2 * plane_size + offset] =
                    static_cast<float>(pixel[2]);
            }
        }

        const std::array<int64_t, 4> input_shape = {
            1,
            3,
            static_cast<int64_t>(pad->input_height),
            static_cast<int64_t>(pad->input_width),
        };

        const auto memory_info =
            Ort::MemoryInfo::CreateCpu(
                OrtArenaAllocator,
                OrtMemTypeDefault
            );

        auto input_tensor = Ort::Value::CreateTensor<float>(
            memory_info,
            tensor.data(),
            tensor.size(),
            input_shape.data(),
            input_shape.size()
        );

        const char* input_names[] = {
            pad->input_name.c_str()
        };
        const char* output_names[] = {
            pad->output_name.c_str()
        };

        auto outputs = pad->session->Run(
            Ort::RunOptions{nullptr},
            input_names,
            &input_tensor,
            1,
            output_names,
            1
        );

        if (outputs.empty() || !outputs[0].IsTensor()) {
            return -4;
        }

        const auto output_info =
            outputs[0].GetTensorTypeAndShapeInfo();
        const size_t count = output_info.GetElementCount();
        if (count < 2) {
            return -5;
        }

        const float* logits =
            outputs[0].GetTensorData<float>();

        float maximum = logits[0];
        for (size_t index = 1; index < count; ++index) {
            maximum = std::max(maximum, logits[index]);
        }

        std::vector<float> exponentials(count);
        float denominator = 0.0F;
        for (size_t index = 0; index < count; ++index) {
            const float value = std::exp(
                logits[index] - maximum
            );
            exponentials[index] = value;
            denominator += value;
        }

        if (denominator <= 0.0F) {
            return -6;
        }

        result->real_probability =
            static_cast<double>(
                exponentials[1] / denominator
            );
        result->input_width = pad->input_width;
        result->input_height = pad->input_height;
        result->class_count = static_cast<int32_t>(count);

        return 0;
    } catch (const Ort::Exception& error) {
        set_last_error(error.what());
        return -7;
    } catch (const cv::Exception& error) {
        set_last_error(error.what());
        return -7;
    } catch (const std::exception& error) {
        set_last_error(error.what());
        return -7;
    } catch (...) {
        set_last_error("unknown PAD inference error");
        return -7;
    }
}

}  // extern "C"
