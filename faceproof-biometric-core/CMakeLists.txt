cmake_minimum_required(VERSION 3.20)

project(faceproof_biometric_core VERSION 0.1.0 LANGUAGES CXX)

option(FACEPROOF_BIOMETRIC_BUILD_TESTS "Build biometric core tests" ON)
option(FACEPROOF_BIOMETRIC_BUILD_CLI "Build biometric math CLI" ON)
option(FACEPROOF_BIOMETRIC_WITH_OPENCV "Build OpenCV biometric vision pipeline" OFF)

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
    endif()
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
