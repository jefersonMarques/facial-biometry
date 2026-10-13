# FaceProof Capture Security

Estado: **implementado, aguardando validação de build/runtime**

Este documento descreve as proteções do protocolo de captura. O navegador ajuda a produzir evidência; a decisão continua no servidor.

## Capture Protocol v2

Cada sessão recebe do servidor:

- `sessionId` único;
- `captureRunId` único;
- `captureChallenge` aleatório;
- `watermarkToken` derivado do challenge;
- expiração curta;
- token HMAC vinculado à sessão e ao run.

O SDK grava uma marca visual da sessão nos JPEGs finais:

```text
FACEPROOF · A1B2C3
```

O código muda a cada sessão.

## Capture Seal

Antes do envio, o SDK calcula SHA-256 dos bytes finais de cada JPEG e cria uma cadeia:

```text
server challenge
      ↓
initial digest
      ↓
frame 0 hash + phase
      ↓
chain hash
      ↓
frame 1 hash + phase
      ↓
chain hash
      ↓
...
      ↓
final chain
```

O servidor recalcula tudo a partir dos bytes recebidos antes de consumir a sessão.

Uma alteração de frame, fase, ordem ou challenge invalida o seal.

### Limite de confiança

O Capture Seal não transforma o browser em ambiente confiável.

Não existe chave secreta fixa dentro do WASM. Uma chave embutida no WASM poderia ser extraída e não seria uma raiz de confiança adequada.

O seal serve para:

- vincular os frames à sessão;
- detectar alteração do material após a geração do seal;
- impedir reutilização direta do pacote de uma sessão em outra;
- criar base para futuras verificações de replay e evidência assinada.

A autoridade continua sendo:

```text
Browser SDK
    ↓
Capture Seal + frames
    ↓
FaceProof Server
    ↓
Secure Core C++
    ↓
decisão
```

## Anti-replay e idempotência

Sessões biométricas continuam single-use.

Além disso, `POST /v1/identity/complete` usa `Idempotency-Key`.

O SDK usa o `captureRunId` como chave.

- mesma chave + mesmo conteúdo após conclusão → retorna o resultado persistido;
- mesma chave + conteúdo diferente → `409 Conflict`;
- sessão já consumida sem receipt compatível → rejeitada.

O receipt é armazenado junto ao check no repositório criptografado.

O SDK pode repetir uma vez a conclusão quando ocorre falha de transporte sem criar uma nova decisão biométrica.

## Marca visual

A marca visual é uma camada complementar, não a defesa principal.

Ela é:

- dinâmica por sessão;
- mostrada durante a captura;
- gravada nos JPEGs enviados;
- incluída indiretamente no SHA-256 dos frames.

Próxima evolução possível: detector server-side para marcas antigas/duplicadas e padrões visuais incompatíveis.

## Aplicativo móvel

No Mobile SDK a mesma ideia poderá receber uma raiz de confiança mais forte:

```text
Capture Seal
    ↓
chave privada do dispositivo
    ↓
Android Keystore / iOS Secure Enclave
    ↓
evidência assinada
```

Isso é especialmente relevante para o modo offline.
