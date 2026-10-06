//go:build linux && cgo

package engine

/*
#cgo CFLAGS: -I${SRCDIR}/../../../../faceproof-biometric-core/include
#cgo LDFLAGS: -ldl

#include <dlfcn.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "faceproof_secure_core.h"

typedef struct FPDynCore {
	void* handle;
	int (*abi_version)(void);
	const char* (*version)(void);
	const char* (*last_error)(void);
	FPSecureCore* (*create)(
		const char*,
		const char*,
		const char*
	);
	void (*destroy)(FPSecureCore*);
	int (*analyze_identity)(
		FPSecureCore*,
		const FPSecureCoreFrameInput*,
		size_t,
		FPSecureCoreIdentityResult*
	);
	int (*reference_jpeg)(
		FPSecureCore*,
		const uint8_t*,
		size_t,
		FPSecureCoreReferenceResult*
	);
	int (*guide_jpeg)(
		FPSecureCore*,
		const uint8_t*,
		size_t,
		FPSecureCoreGuideResult*
	);
} FPDynCore;

static void fp_loader_error(
	char* output,
	size_t capacity,
	const char* message
) {
	if (output == NULL || capacity == 0) {
		return;
	}
	if (message == NULL) {
		message = "unknown loader error";
	}
	snprintf(output, capacity, "%s", message);
}

static FPDynCore* fp_dyn_open(
	const char* path,
	char* error_output,
	size_t error_capacity
) {
	if (path == NULL || path[0] == '\0') {
		fp_loader_error(
			error_output,
			error_capacity,
			"Secure Core library path is empty"
		);
		return NULL;
	}

	void* handle = dlopen(path, RTLD_NOW | RTLD_LOCAL);
	if (handle == NULL) {
		fp_loader_error(error_output, error_capacity, dlerror());
		return NULL;
	}

	FPDynCore* api = (FPDynCore*)calloc(1, sizeof(FPDynCore));
	if (api == NULL) {
		fp_loader_error(
			error_output,
			error_capacity,
			"could not allocate Secure Core loader"
		);
		dlclose(handle);
		return NULL;
	}
	api->handle = handle;

#define FP_LOAD(field, symbol_name)                                      \
	do {                                                                  \
		dlerror();                                                          \
		*(void**)(&api->field) = dlsym(handle, symbol_name);                 \
		const char* symbol_error = dlerror();                                \
		if (symbol_error != NULL) {                                         \
			fp_loader_error(error_output, error_capacity, symbol_error);       \
			dlclose(handle);                                                   \
			free(api);                                                         \
			return NULL;                                                       \
		}                                                                   \
	} while (0)

	FP_LOAD(abi_version, "fp_secure_core_abi_version");
	FP_LOAD(version, "fp_secure_core_version");
	FP_LOAD(last_error, "fp_secure_core_last_error");
	FP_LOAD(create, "fp_secure_core_create");
	FP_LOAD(destroy, "fp_secure_core_destroy");
	FP_LOAD(analyze_identity, "fp_secure_core_analyze_identity");
	FP_LOAD(reference_jpeg, "fp_secure_core_reference_jpeg");
	FP_LOAD(guide_jpeg, "fp_secure_core_guide_jpeg");

#undef FP_LOAD

	return api;
}

static void fp_dyn_close(FPDynCore* api) {
	if (api == NULL) {
		return;
	}
	if (api->handle != NULL) {
		dlclose(api->handle);
	}
	free(api);
}

static int fp_dyn_abi_version(FPDynCore* api) {
	return api == NULL ? -1 : api->abi_version();
}

static const char* fp_dyn_version(FPDynCore* api) {
	return api == NULL ? "" : api->version();
}

static const char* fp_dyn_last_error(FPDynCore* api) {
	return api == NULL ? "" : api->last_error();
}

static FPSecureCore* fp_dyn_create(
	FPDynCore* api,
	const char* yunet,
	const char* sface,
	const char* minifas
) {
	return api == NULL ? NULL : api->create(yunet, sface, minifas);
}

static void fp_dyn_destroy(FPDynCore* api, FPSecureCore* core) {
	if (api != NULL && core != NULL) {
		api->destroy(core);
	}
}

static int fp_dyn_analyze_identity(
	FPDynCore* api,
	FPSecureCore* core,
	const FPSecureCoreFrameInput* frames,
	size_t frame_count,
	FPSecureCoreIdentityResult* result
) {
	if (api == NULL) {
		return -999;
	}
	return api->analyze_identity(core, frames, frame_count, result);
}

static int fp_dyn_reference_jpeg(
	FPDynCore* api,
	FPSecureCore* core,
	const uint8_t* data,
	size_t size,
	FPSecureCoreReferenceResult* result
) {
	if (api == NULL) {
		return -999;
	}
	return api->reference_jpeg(core, data, size, result);
}

static int fp_dyn_guide_jpeg(
	FPDynCore* api,
	FPSecureCore* core,
	const uint8_t* data,
	size_t size,
	FPSecureCoreGuideResult* result
) {
	if (api == NULL) {
		return -999;
	}
	return api->guide_jpeg(core, data, size, result);
}
*/
import "C"

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"unsafe"

	"faceproof/services/api/internal/domain"
)

const secureCoreABIVersion = 1

type NativeClient struct {
	api       *C.FPDynCore
	core      *C.FPSecureCore
	version   string
	closeOnce sync.Once
}

func NewNativeClient(configuration NativeConfig) (*NativeClient, error) {
	if strings.TrimSpace(configuration.LibraryPath) == "" {
		return nil, errors.New("Secure Core library path is required")
	}
	if strings.TrimSpace(configuration.YUNetModelPath) == "" ||
		strings.TrimSpace(configuration.SFaceModelPath) == "" ||
		strings.TrimSpace(configuration.MiniFASNetPath) == "" {
		return nil, errors.New("Secure Core model paths are required")
	}

	libraryPath := C.CString(configuration.LibraryPath)
	defer C.free(unsafe.Pointer(libraryPath))

	errorBuffer := (*C.char)(C.calloc(512, 1))
	if errorBuffer == nil {
		return nil, errors.New("could not allocate Secure Core loader error buffer")
	}
	defer C.free(unsafe.Pointer(errorBuffer))

	api := C.fp_dyn_open(libraryPath, errorBuffer, 512)
	if api == nil {
		return nil, fmt.Errorf(
			"load Secure Core: %s",
			strings.TrimSpace(C.GoString(errorBuffer)),
		)
	}

	if abi := int(C.fp_dyn_abi_version(api)); abi != secureCoreABIVersion {
		C.fp_dyn_close(api)
		return nil, fmt.Errorf(
			"Secure Core ABI mismatch: library=%d expected=%d",
			abi,
			secureCoreABIVersion,
		)
	}

	yunet := C.CString(configuration.YUNetModelPath)
	sface := C.CString(configuration.SFaceModelPath)
	minifas := C.CString(configuration.MiniFASNetPath)
	defer C.free(unsafe.Pointer(yunet))
	defer C.free(unsafe.Pointer(sface))
	defer C.free(unsafe.Pointer(minifas))

	core := C.fp_dyn_create(api, yunet, sface, minifas)
	if core == nil {
		message := strings.TrimSpace(C.GoString(C.fp_dyn_last_error(api)))
		C.fp_dyn_close(api)
		if message == "" {
			message = "unknown initialization error"
		}
		return nil, fmt.Errorf("initialize Secure Core: %s", message)
	}

	client := &NativeClient{
		api:     api,
		core:    core,
		version: C.GoString(C.fp_dyn_version(api)),
	}
	return client, nil
}

func (client *NativeClient) Close() error {
	if client == nil {
		return nil
	}
	client.closeOnce.Do(func() {
		if client.api != nil && client.core != nil {
			C.fp_dyn_destroy(client.api, client.core)
			client.core = nil
		}
		if client.api != nil {
			C.fp_dyn_close(client.api)
			client.api = nil
		}
	})
	return nil
}

func (client *NativeClient) AnalyzeIdentity(
	ctx context.Context,
	request domain.EngineIdentityRequest,
) (domain.EngineResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.EngineResult{}, err
	}
	if client == nil || client.api == nil || client.core == nil {
		return domain.EngineResult{}, errors.New("Secure Core is not initialized")
	}
	if len(request.GuidedFrames) < 6 || len(request.GuidedFrames) > 12 {
		return domain.EngineResult{}, errors.New(
			"identity capture must contain between 6 and 12 frames",
		)
	}

	frameSize := C.size_t(unsafe.Sizeof(C.FPSecureCoreFrameInput{}))
	frameMemory := C.calloc(C.size_t(len(request.GuidedFrames)), frameSize)
	if frameMemory == nil {
		return domain.EngineResult{}, errors.New("could not allocate native frame manifest")
	}
	defer C.free(frameMemory)

	nativeFrames := unsafe.Slice(
		(*C.FPSecureCoreFrameInput)(frameMemory),
		len(request.GuidedFrames),
	)
	allocated := make([]unsafe.Pointer, 0, len(request.GuidedFrames))
	defer func() {
		for _, pointer := range allocated {
			C.free(pointer)
		}
	}()

	for index, frame := range request.GuidedFrames {
		if len(frame.ImageBytes) == 0 {
			return domain.EngineResult{}, errors.New(
				"native identity capture requires binary JPEG frames",
			)
		}

		phase, err := nativePhase(frame.Phase)
		if err != nil {
			return domain.EngineResult{}, err
		}
		data := C.CBytes(frame.ImageBytes)
		if data == nil {
			return domain.EngineResult{}, errors.New("could not allocate native JPEG")
		}
		allocated = append(allocated, data)

		nativeFrames[index].data = (*C.uint8_t)(data)
		nativeFrames[index].size = C.size_t(len(frame.ImageBytes))
		nativeFrames[index].phase = C.int32_t(phase)
	}

	var nativeResult C.FPSecureCoreIdentityResult
	code := int(C.fp_dyn_analyze_identity(
		client.api,
		client.core,
		(*C.FPSecureCoreFrameInput)(frameMemory),
		C.size_t(len(request.GuidedFrames)),
		&nativeResult,
	))
	if code != 0 {
		return domain.EngineResult{}, client.nativeError("analyze identity", code)
	}
	if err := ctx.Err(); err != nil {
		return domain.EngineResult{}, err
	}

	embeddingSize := int(nativeResult.embedding_size)
	if embeddingSize <= 0 ||
		embeddingSize > int(C.FP_SECURE_CORE_EMBEDDING_CAPACITY) {
		return domain.EngineResult{}, errors.New("Secure Core returned invalid embedding size")
	}

	embedding := make([]float64, embeddingSize)
	for index := 0; index < embeddingSize; index++ {
		embedding[index] = float64(nativeResult.embedding[index])
	}

	selectedCount := int(nativeResult.selected_embedding_count)
	if selectedCount < 0 ||
		selectedCount > int(C.FP_SECURE_CORE_MAX_SELECTED_FRAMES) {
		return domain.EngineResult{}, errors.New(
			"Secure Core returned invalid selected embedding count",
		)
	}

	faceEmbeddings := make([]domain.EngineFaceEmbedding, 0, selectedCount)
	for index := 0; index < selectedCount; index++ {
		selected := nativeResult.selected_embeddings[index]
		size := int(selected.embedding_size)
		if size <= 0 ||
			size > int(C.FP_SECURE_CORE_EMBEDDING_CAPACITY) {
			return domain.EngineResult{}, errors.New(
				"Secure Core returned invalid selected embedding size",
			)
		}

		values := make([]float64, size)
		for embeddingIndex := 0; embeddingIndex < size; embeddingIndex++ {
			values[embeddingIndex] = float64(selected.embedding[embeddingIndex])
		}
		faceEmbeddings = append(faceEmbeddings, domain.EngineFaceEmbedding{
			FrameIndex: int(selected.frame_index),
			Phase:      phaseName(int(selected.phase)),
			Quality:    float64(selected.quality),
			Embedding:  values,
		})
	}

	passiveStatus := "unavailable"
	if int(nativeResult.passive_pad_available) != 0 {
		passiveStatus = "available"
	}

	diagnostics := nativeDiagnostics(
		client.version,
		float64(nativeResult.quality_score),
		float64(nativeResult.face_presence),
		float64(nativeResult.guided_capture_score),
		float64(nativeResult.temporal_motion_score),
	)

	return domain.EngineResult{
		LivenessScore: float64(nativeResult.liveness_score),
		PassivePAD: domain.EngineSignal{
			Score:  float64(nativeResult.passive_pad_score),
			Status: passiveStatus,
		},
		TemporalMotion: domain.EngineSignal{
			Score:  float64(nativeResult.temporal_motion_score),
			Status: "available",
		},
		Illumination: domain.EngineSignal{
			Score:  0.5,
			Status: "not_used",
		},
		GuidedCapture: domain.EngineSignal{
			Score:  float64(nativeResult.guided_capture_score),
			Status: "available",
		},
		Quality: domain.EngineQuality{
			Score:           float64(nativeResult.quality_score),
			FacePresence:    float64(nativeResult.face_presence),
			Sharpness:       float64(nativeResult.sharpness),
			Brightness:      float64(nativeResult.brightness),
			FaceSize:        float64(nativeResult.face_size),
			DetectedFrames:  int(nativeResult.detected_frames),
			ProcessedFrames: int(nativeResult.processed_frames),
		},
		Embedding:      embedding,
		FaceEmbeddings: faceEmbeddings,
		EmbeddingModel: "sface",
		BestFrameIndex: int(nativeResult.best_frame_index),
		Diagnostics:    diagnostics,
		NativeShadow: &domain.NativeShadowComparison{
			Status:       "authority",
			NativeFrames: int(nativeResult.detected_frames),
		},
	}, nil
}

func (client *NativeClient) ExtractReference(
	ctx context.Context,
	imagePayload string,
) (domain.ReferenceResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.ReferenceResult{}, err
	}
	image, err := decodeImagePayload(imagePayload)
	if err != nil {
		return domain.ReferenceResult{}, err
	}
	if client == nil || client.api == nil || client.core == nil {
		return domain.ReferenceResult{}, errors.New("Secure Core is not initialized")
	}

	data := C.CBytes(image)
	if data == nil {
		return domain.ReferenceResult{}, errors.New("could not allocate reference JPEG")
	}
	defer C.free(data)

	var nativeResult C.FPSecureCoreReferenceResult
	code := int(C.fp_dyn_reference_jpeg(
		client.api,
		client.core,
		(*C.uint8_t)(data),
		C.size_t(len(image)),
		&nativeResult,
	))
	if code != 0 {
		return domain.ReferenceResult{}, client.nativeError("extract reference", code)
	}

	embeddingSize := int(nativeResult.embedding_size)
	variantCount := int(nativeResult.variant_count)
	if embeddingSize <= 0 ||
		embeddingSize > int(C.FP_SECURE_CORE_EMBEDDING_CAPACITY) ||
		variantCount <= 0 ||
		variantCount > int(C.FP_SECURE_CORE_MAX_SELECTED_FRAMES) {
		return domain.ReferenceResult{}, errors.New(
			"Secure Core returned invalid reference embedding dimensions",
		)
	}

	embedding := make([]float64, embeddingSize)
	for index := 0; index < embeddingSize; index++ {
		embedding[index] = float64(nativeResult.embedding[index])
	}

	variants := make([][]float64, variantCount)
	for variantIndex := 0; variantIndex < variantCount; variantIndex++ {
		variants[variantIndex] = make([]float64, embeddingSize)
		for embeddingIndex := 0; embeddingIndex < embeddingSize; embeddingIndex++ {
			variants[variantIndex][embeddingIndex] = float64(
				nativeResult.variants[variantIndex][embeddingIndex],
			)
		}
	}

	return domain.ReferenceResult{
		Embedding:      embedding,
		Embeddings:     variants,
		EmbeddingModel: "sface",
		Quality: domain.EngineQuality{
			Score:           float64(nativeResult.quality.score),
			FacePresence:    1,
			Sharpness:       float64(nativeResult.quality.sharpness),
			Brightness:      float64(nativeResult.quality.brightness),
			FaceSize:        float64(nativeResult.quality.face_size),
			DetectedFrames:  1,
			ProcessedFrames: 1,
		},
	}, nil
}

func (client *NativeClient) Guide(
	ctx context.Context,
	imagePayload string,
) (domain.EngineGuideResult, error) {
	if err := ctx.Err(); err != nil {
		return domain.EngineGuideResult{}, err
	}
	image, err := decodeImagePayload(imagePayload)
	if err != nil {
		return domain.EngineGuideResult{}, err
	}
	if client == nil || client.api == nil || client.core == nil {
		return domain.EngineGuideResult{}, errors.New("Secure Core is not initialized")
	}

	data := C.CBytes(image)
	if data == nil {
		return domain.EngineGuideResult{}, errors.New("could not allocate guide JPEG")
	}
	defer C.free(data)

	var nativeResult C.FPSecureCoreGuideResult
	code := int(C.fp_dyn_guide_jpeg(
		client.api,
		client.core,
		(*C.uint8_t)(data),
		C.size_t(len(image)),
		&nativeResult,
	))
	if code != 0 {
		return domain.EngineGuideResult{}, client.nativeError("guide face", code)
	}

	if int(nativeResult.face_detected) == 0 {
		return domain.EngineGuideResult{
			FaceDetected: false,
			Quality: domain.EngineQuality{
				ProcessedFrames: 1,
			},
		}, nil
	}

	return domain.EngineGuideResult{
		FaceDetected: true,
		Confidence:   float64(nativeResult.confidence),
		CenterX:      float64(nativeResult.center_x),
		CenterY:      float64(nativeResult.center_y),
		WidthRatio:   float64(nativeResult.width_ratio),
		HeightRatio:  float64(nativeResult.height_ratio),
		RollDegrees:  float64(nativeResult.roll_degrees),
		Quality: domain.EngineQuality{
			Score:           float64(nativeResult.quality.score),
			FacePresence:    1,
			Sharpness:       float64(nativeResult.quality.sharpness),
			Brightness:      float64(nativeResult.quality.brightness),
			FaceSize:        float64(nativeResult.quality.face_size),
			DetectedFrames:  1,
			ProcessedFrames: 1,
		},
	}, nil
}

func (client *NativeClient) nativeError(operation string, code int) error {
	message := ""
	if client != nil && client.api != nil {
		message = strings.TrimSpace(C.GoString(C.fp_dyn_last_error(client.api)))
	}
	if message == "" {
		message = "unknown Secure Core error"
	}
	return fmt.Errorf("%s: code=%d: %s", operation, code, message)
}

func nativePhase(value string) (int, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "far":
		return int(C.FP_SECURE_PHASE_FAR), nil
	case "near":
		return int(C.FP_SECURE_PHASE_NEAR), nil
	default:
		return 0, fmt.Errorf("invalid guided frame phase %q", value)
	}
}

func phaseName(value int) string {
	switch value {
	case int(C.FP_SECURE_PHASE_FAR):
		return "far"
	case int(C.FP_SECURE_PHASE_NEAR):
		return "near"
	default:
		return "unknown"
	}
}

func decodeImagePayload(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("image payload is empty")
	}

	if strings.HasPrefix(value, "data:") {
		comma := strings.IndexByte(value, ',')
		if comma < 0 || comma == len(value)-1 {
			return nil, errors.New("invalid image data URL")
		}
		metadata := value[:comma]
		if !strings.Contains(metadata, ";base64") {
			return nil, errors.New("image data URL must use base64")
		}
		value = value[comma+1:]
	}

	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("decode image payload: %w", err)
	}
	if len(decoded) == 0 {
		return nil, errors.New("decoded image payload is empty")
	}
	return decoded, nil
}

func nativeDiagnostics(
	version string,
	quality float64,
	facePresence float64,
	guided float64,
	temporal float64,
) []string {
	diagnostics := []string{
		"Secure Core C++ authority " + strings.TrimSpace(version),
	}
	if quality < 0.50 {
		diagnostics = append(diagnostics, "capture quality is low")
	}
	if facePresence < 0.80 {
		diagnostics = append(diagnostics, "face presence is low")
	}
	if guided < 0.45 {
		diagnostics = append(
			diagnostics,
			"guided far/near capture signal is weak",
		)
	}
	if temporal < 0.20 {
		diagnostics = append(diagnostics, "temporal motion signal is weak")
	}
	return diagnostics
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
