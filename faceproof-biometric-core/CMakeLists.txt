cmake_minimum_required(VERSION 3.20)

project(faceproof_biometric_core VERSION 0.2.0 LANGUAGES CXX)

set(CMAKE_POSITION_INDEPENDENT_CODE ON)

option(FACEPROOF_BIOMETRIC_BUILD_TESTS "Build biometric core tests" ON)
option(FACEPROOF_BIOMETRIC_BUILD_CLI "Build biometric math CLI" ON)
option(FACEPROOF_BIOMETRIC_WITH_OPENCV "Build OpenCV biometric vision pipeline" OFF)
option(FACEPROOF_BIOMETRIC_WITH_ONNXRUNTIME "Build ONNX Runtime passive PAD pipeline" OFF)
set(FACEPROOF_ONNXRUNTIME_ROOT "" CACHE PATH "ONNX Runtime native distribution root")

add_library(faceproof_biometric_core STATIC
    src/scoring.cpp
)

target_include_directories(faceproof_biometric_core
    PUBLIC
        ${CMAKE_CURRENT_SOURCE_DIR}/include
)

target_compile_features(faceproof_biometric_core PUBLIC cxx_std_17)

if(MSVC)
    target_compile_options(faceproof_biometric_core PRIVATE /W4 /permissive-)
else()
    target_compile_options(faceproof_biometric_core PRIVATE -Wall -Wextra -Wpedantic)
endif()

if(FACEPROOF_BIOMETRIC_BUILD_CLI)
    add_executable(faceproof_biometric_math_cli
        tools/math_cli.cpp
    )
    target_link_libraries(faceproof_biometric_math_cli PRIVATE faceproof_biometric_core)
endif()

if(FACEPROOF_BIOMETRIC_WITH_OPENCV)
    find_package(OpenCV REQUIRED COMPONENTS core imgcodecs imgproc objdetect)

    add_library(faceproof_biometric_vision STATIC
        src/opencv_vision.cpp
    )

    target_include_directories(faceproof_biometric_vision
        PUBLIC
            ${CMAKE_CURRENT_SOURCE_DIR}/include
            ${OpenCV_INCLUDE_DIRS}
    )

    target_compile_features(faceproof_biometric_vision PUBLIC cxx_std_17)
    target_link_libraries(faceproof_biometric_vision PUBLIC ${OpenCV_LIBS})

    if(MSVC)
        target_compile_options(faceproof_biometric_vision PRIVATE /W4 /permissive-)
    else()
        target_compile_options(faceproof_biometric_vision PRIVATE -Wall -Wextra -Wpedantic)
    endif()

    if(FACEPROOF_BIOMETRIC_BUILD_CLI)
        add_executable(faceproof_biometric_vision_cli
            tools/vision_cli.cpp
        )
        target_link_libraries(
            faceproof_biometric_vision_cli
            PRIVATE
                faceproof_biometric_vision
        )

        add_executable(faceproof_biometric_reference_cli
            tools/reference_cli.cpp
        )
        target_link_libraries(
            faceproof_biometric_reference_cli
            PRIVATE
                faceproof_biometric_vision
        )
    endif()
endif()

if(FACEPROOF_BIOMETRIC_WITH_ONNXRUNTIME)
    if(NOT FACEPROOF_BIOMETRIC_WITH_OPENCV)
        message(FATAL_ERROR "FACEPROOF_BIOMETRIC_WITH_ONNXRUNTIME requires FACEPROOF_BIOMETRIC_WITH_OPENCV")
    endif()
    if(NOT FACEPROOF_ONNXRUNTIME_ROOT)
        message(FATAL_ERROR "FACEPROOF_ONNXRUNTIME_ROOT is required")
    endif()

    find_path(
        ONNXRUNTIME_INCLUDE_DIR
        onnxruntime_cxx_api.h
        PATHS "${FACEPROOF_ONNXRUNTIME_ROOT}/include"
        NO_DEFAULT_PATH
    )
    find_library(
        ONNXRUNTIME_LIBRARY
        NAMES onnxruntime
        PATHS "${FACEPROOF_ONNXRUNTIME_ROOT}/lib"
        NO_DEFAULT_PATH
    )

    if(NOT ONNXRUNTIME_INCLUDE_DIR OR NOT ONNXRUNTIME_LIBRARY)
        message(FATAL_ERROR "Pinned ONNX Runtime headers/library not found")
    endif()

    add_library(faceproof_biometric_pad STATIC
        src/onnx_pad.cpp
    )
    target_include_directories(faceproof_biometric_pad
        PUBLIC
            ${CMAKE_CURRENT_SOURCE_DIR}/include
            ${ONNXRUNTIME_INCLUDE_DIR}
            ${OpenCV_INCLUDE_DIRS}
    )
    target_compile_features(faceproof_biometric_pad PUBLIC cxx_std_17)
    target_link_libraries(
        faceproof_biometric_pad
        PUBLIC
            faceproof_biometric_vision
            ${ONNXRUNTIME_LIBRARY}
    )

    if(MSVC)
        target_compile_options(faceproof_biometric_pad PRIVATE /W4 /permissive-)
    else()
        target_compile_options(faceproof_biometric_pad PRIVATE -Wall -Wextra -Wpedantic)
    endif()

    if(FACEPROOF_BIOMETRIC_BUILD_CLI)
        add_executable(faceproof_biometric_pad_cli
            tools/pad_cli.cpp
        )
        target_link_libraries(
            faceproof_biometric_pad_cli
            PRIVATE
                faceproof_biometric_pad
        )
    endif()

    add_library(faceproof_secure_core SHARED
        src/secure_core.cpp
    )
    target_include_directories(faceproof_secure_core
        PUBLIC
            ${CMAKE_CURRENT_SOURCE_DIR}/include
            ${OpenCV_INCLUDE_DIRS}
            ${ONNXRUNTIME_INCLUDE_DIR}
    )
    target_compile_features(faceproof_secure_core PUBLIC cxx_std_17)
    target_link_libraries(
        faceproof_secure_core
        PRIVATE
            faceproof_biometric_core
            faceproof_biometric_vision
            faceproof_biometric_pad
    )
    set_target_properties(faceproof_secure_core PROPERTIES
        OUTPUT_NAME faceproof_core
        VERSION ${PROJECT_VERSION}
        SOVERSION 1
    )

    if(MSVC)
        target_compile_options(faceproof_secure_core PRIVATE /W4 /permissive-)
    else()
        target_compile_options(faceproof_secure_core PRIVATE -Wall -Wextra -Wpedantic)
    endif()

    install(
        TARGETS faceproof_secure_core
        LIBRARY DESTINATION lib
    )
    install(
        FILES
            include/faceproof_biometric.h
            include/faceproof_biometric_vision.h
            include/faceproof_biometric_pad.h
            include/faceproof_secure_core.h
        DESTINATION include
    )
endif()

if(FACEPROOF_BIOMETRIC_BUILD_TESTS)
    enable_testing()
    add_executable(faceproof_biometric_core_test
        tests/scoring_test.cpp
    )
    target_link_libraries(faceproof_biometric_core_test PRIVATE faceproof_biometric_core)
    add_test(
        NAME faceproof_biometric_core_test
        COMMAND faceproof_biometric_core_test
    )
endif()
