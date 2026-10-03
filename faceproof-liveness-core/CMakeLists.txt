cmake_minimum_required(VERSION 3.20)

project(faceproof_liveness_core VERSION 0.1.0 LANGUAGES CXX)

option(FACEPROOF_BUILD_TESTS "Build native liveness core tests" ON)

add_library(faceproof_liveness_core STATIC
    src/geometry.cpp
    src/liveness_core.cpp
    src/math.cpp
    src/summarize.cpp
)

target_include_directories(faceproof_liveness_core
    PUBLIC
        ${CMAKE_CURRENT_SOURCE_DIR}/include
    PRIVATE
        ${CMAKE_CURRENT_SOURCE_DIR}/src
)

target_compile_features(faceproof_liveness_core PUBLIC cxx_std_17)

if(MSVC)
    target_compile_options(faceproof_liveness_core PRIVATE /W4 /permissive-)
else()
    target_compile_options(faceproof_liveness_core PRIVATE -Wall -Wextra -Wpedantic)
endif()

if(EMSCRIPTEN)
    add_executable(faceproof_liveness_wasm
        wasm/bindings.cpp
    )

    target_link_libraries(faceproof_liveness_wasm PRIVATE faceproof_liveness_core)

    set_target_properties(faceproof_liveness_wasm PROPERTIES
        OUTPUT_NAME "faceproof-liveness-core"
        SUFFIX ".js"
    )

    target_compile_options(faceproof_liveness_core PRIVATE
        -O3
        -msimd128
        -fno-exceptions
        -fno-rtti
    )

    target_compile_options(faceproof_liveness_wasm PRIVATE
        -O3
        -msimd128
        -fno-exceptions
        -fno-rtti
    )

    target_link_options(faceproof_liveness_wasm PRIVATE
        -O3
        -msimd128
        -sMODULARIZE=1
        -sEXPORT_ES6=1
        -sENVIRONMENT=web,worker
        -sALLOW_MEMORY_GROWTH=1
        -sFILESYSTEM=0
        -sNO_EXIT_RUNTIME=1
        "-sEXPORTED_FUNCTIONS=['_malloc','_free']"
    )
endif()

if(FACEPROOF_BUILD_TESTS AND NOT EMSCRIPTEN)
    enable_testing()

    add_executable(faceproof_liveness_core_test
        tests/liveness_core_test.cpp
    )

    target_link_libraries(faceproof_liveness_core_test PRIVATE faceproof_liveness_core)

    add_test(
        NAME faceproof_liveness_core_test
        COMMAND faceproof_liveness_core_test
    )
endif()
