const SAMPLE_STEP = 4;
const MIN_BRIGHTNESS = 0.16;
const MAX_BRIGHTNESS = 0.88;
const MIN_CONTRAST = 0.055;
const MIN_SHARPNESS = 0.018;
export class FrameQualityAnalyzer {
    analyze(imageData) {
        const { brightness, contrast, sharpness } = this.measure(imageData);
        const issue = this.issueFor(brightness, contrast, sharpness);
        return {
            brightness,
            contrast,
            sharpness,
            acceptable: issue === null,
            issue,
        };
    }
    measure(imageData) {
        const samples = [];
        let gradientTotal = 0;
        let gradientCount = 0;
        for (let y = 0; y < imageData.height; y += SAMPLE_STEP) {
            for (let x = 0; x < imageData.width; x += SAMPLE_STEP) {
                const current = luminanceAt(imageData, x, y);
                samples.push(current);
                if (x + SAMPLE_STEP < imageData.width) {
                    gradientTotal += Math.abs(current - luminanceAt(imageData, x + SAMPLE_STEP, y));
                    gradientCount++;
                }
                if (y + SAMPLE_STEP < imageData.height) {
                    gradientTotal += Math.abs(current - luminanceAt(imageData, x, y + SAMPLE_STEP));
                    gradientCount++;
                }
            }
        }
        if (samples.length === 0) {
            return { brightness: 0, contrast: 0, sharpness: 0 };
        }
        const brightness = mean(samples);
        const variance = mean(samples.map((value) => (value - brightness) ** 2));
        const contrast = Math.sqrt(variance);
        const sharpness = gradientCount > 0 ? gradientTotal / gradientCount : 0;
        return {
            brightness: clamp01(brightness),
            contrast: clamp01(contrast),
            sharpness: clamp01(sharpness),
        };
    }
    issueFor(brightness, contrast, sharpness) {
        if (brightness < MIN_BRIGHTNESS) {
            return "Ambiente muito escuro. Aumente a iluminação do rosto.";
        }
        if (brightness > MAX_BRIGHTNESS) {
            return "Luz excessiva. Evite uma fonte forte diretamente no rosto.";
        }
        if (contrast < MIN_CONTRAST) {
            return "Imagem com pouco contraste. Procure uma iluminação mais uniforme.";
        }
        if (sharpness < MIN_SHARPNESS) {
            return "Imagem pouco nítida. Mantenha o aparelho firme e limpe a câmera.";
        }
        return null;
    }
}
function luminanceAt(imageData, x, y) {
    const index = (y * imageData.width + x) * 4;
    const red = imageData.data[index] ?? 0;
    const green = imageData.data[index + 1] ?? 0;
    const blue = imageData.data[index + 2] ?? 0;
    return (0.2126 * red + 0.7152 * green + 0.0722 * blue) / 255;
}
function mean(values) {
    return values.reduce((total, value) => total + value, 0) / Math.max(values.length, 1);
}
function clamp01(value) {
    return Math.max(0, Math.min(1, value));
}
//# sourceMappingURL=frame-quality.js.map