# FaceProof Production Plan

Estado: **Production Candidate em construção**

Regra: um item recebe `✅` somente quando está implementado e validado. `⏳` indica trabalho ativo e `⬜` ainda não iniciado.

## Roadmap

1. ✅ **Congelar baseline técnico**
   - Baseline de código: `c2b150edef53df5c91d701616f6ef9c4e8f63f2f`
   - SDK: `0.3.0`
   - Liveness Core/WASM: `0.1.0`
   - Match threshold: `0.363`
   - Liveness threshold: `0.68`
   - Review threshold: `0.55`
   - Passive PAD obrigatório: `true`
   - Modelos: YuNet 2023mar, SFace 2021dec, MiniFASNetV2
   - WASM SHA-256: `b14f9269139f0c24248d44eaec2b0c47ea2e2888653593b06558fbbb6d24b935`
   - Baseline de validação em 06/10/2026: 9 verificações concluídas, 9 Secure Core `ok`, 0 `partial`, 0 `drift`.

2. ⏳ **Linux como única plataforma de servidor**
   - Remover Windows do CI.
   - Remover scripts PowerShell de desenvolvimento.
   - Produção alvo: Linux x86_64.

3. ⏳ **C++ como autoridade biométrica**
   - Python deixa de decidir o fluxo de identidade.
   - O resultado usado pelo Go deve vir do Secure Core C++.
   - Python permanece temporariamente como referência/shadow.

4. ⏳ **Secure Core como biblioteca nativa persistente**
   - Produzir `libfaceproof_core.so`.
   - ABI C pequena, versionada e estável.
   - Contexto carrega YuNet, SFace e MiniFASNet uma única vez.
   - Go deve consumir a biblioteca sem spawn de CLI por captura.

5. ⬜ **Unificar pipeline biométrico no Secure Core**
   - YuNet → quality → PAD → SFace → embeddings → fusion → scoring.

6. ⬜ **Retirar Python do runtime**
   - Sem `engine_server.py`, venv, NumPy, OpenCV Python ou ONNX Runtime Python no runtime normal.
   - Python somente em testes, validação e R&D.

7. ⬜ **FaceProof Model Pack**
   - Manifesto versionado.
   - Hashes de todos os modelos.
   - Assinatura do pacote.
   - Preparar proteção adicional dos artefatos.

8. ⬜ **Hardening de segurança**
   - Sessões curtas, single-use, runId server-generated, anti-replay, ordem/timing das fases, rate limiting, limites e isolamento por tenant.

9. ⬜ **Separar SDK público do núcleo privado**
   - Browser filtra/coleta.
   - Servidor valida/decide.
   - Nenhum segredo crítico no WASM.

10. ⬜ **Observabilidade e analytics duráveis**
    - Outbox/spool fora do caminho crítico.
    - Métricas e eventos completos.
    - Relatórios técnicos reproduzíveis.

11. ⬜ **Testes automatizados de produção**
    - Unit, golden samples, parity, API, replay, malformed payload, timeout, concorrência, memória, carga e recovery.

12. ⬜ **Bateria biométrica formal — gate Private Beta**
    - Meta inicial: 1.500–2.000 execuções classificadas.

13. ⬜ **Dataset de validação**
    - Cenário, dispositivo, câmera, iluminação, tentativas, PAD, liveness, face, quality, tempo, versões e resultado esperado/obtido.

14. ⬜ **FaceProof Production Candidate**
    - Browser → HTTPS → Go API → libfaceproof_core.so → decisão → audit.

15. ⬜ **Private Beta**
    - 1–3 clientes controlados.
    - Monitoramento forte e thresholds congelados.

16. ⬜ **FaceProof Production v1**
    - Meta interna: ~7.000–10.000 verificações bem classificadas + estabilidade operacional + nenhum bypass conhecido.

17. ⬜ **Arquitetura comercial**
    - FaceProof Cloud.
    - FaceProof Private Cloud.
    - FaceProof Enterprise / On-premise.

18. ⬜ **Proteção comercial/IP**
    - Cliente recebe SDK/API/resultados; não recebe código C++, pesos abertos, thresholds, fusion, heurísticas ou regras antifraude.

19. ⬜ **Licenciamento e tenant**
    - API key, tenant, domínio autorizado, quota, feature flags, versionamento e billing.

20. ⬜ **Validação independente**
    - Metodologia PAD formal, benchmark externo e preparação para ISO/IEC 30107-3.

## Baseline técnico congelado

O baseline acima é referência para comparar a migração de Python authority → C++ authority. Durante essa migração, thresholds e modelos biométricos não devem ser alterados simultaneamente. Qualquer mudança posterior deve gerar uma nova versão de baseline.
