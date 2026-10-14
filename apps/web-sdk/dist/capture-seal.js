const CAPTURE_SEAL_VERSION = "1";
export async function buildCaptureSeal(session, frames) {
    if (!session.captureChallenge) {
        throw new Error("Capture session challenge is missing");
    }
    let chain = await sha256Hex(utf8([
        "faceproof-capture-seal-v1",
        session.sessionId,
        session.captureRunId,
        session.captureChallenge,
    ].join("|")));
    const frameSha256 = [];
    for (let index = 0; index < frames.length; index++) {
        const frame = frames[index];
        if (!frame) {
            throw new Error("Capture frame is missing");
        }
        const frameHash = await sha256Hex(await frame.imageBlob.arrayBuffer());
        frameSha256.push(frameHash);
        chain = await sha256Hex(utf8([
            chain,
            String(index),
            frame.phase.trim().toLowerCase(),
            frameHash.toLowerCase(),
        ].join("|")));
    }
    return {
        version: CAPTURE_SEAL_VERSION,
        challenge: session.captureChallenge,
        frameSha256,
        chainSha256: chain,
    };
}
async function sha256Hex(data) {
    const digest = await crypto.subtle.digest("SHA-256", data);
    return Array.from(new Uint8Array(digest))
        .map((value) => value.toString(16).padStart(2, "0"))
        .join("");
}
function utf8(value) {
    const encoded = new TextEncoder().encode(value);
    const copy = new Uint8Array(encoded.byteLength);
    copy.set(encoded);
    return copy.buffer;
}
