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

2. ✅ **Linux como única plataforma de servidor**
   - Windows removido do CI.
   - Scripts PowerShell de desenvolvimento removidos.
   - Produção alvo: Linux x86_64.

3. ✅ **C++ como autoridade biométrica**
   - ✅ Go pode encaminhar reference, guide e identity analyze ao Secure Core nativo.
   - ✅ Modo explícito `--native-core` criado.
   - ✅ API falha no startup se o Secure Core configurado não puder ser carregado.
   - ✅ Analytics/UI distinguem C++ authority como `AUTH`.
   - ✅ Startup Linux validado com `--native-core`: API configurada para FaceProof Secure Core C++ authority.
   - ✅ Captura real validada com C++ como autoridade.
   - Python permanece temporariamente como fallback/referência até o item 6.

4. ✅ **Secure Core como biblioteca nativa persistente**
   - ✅ `libfaceproof_core.so` criada e compilando no CI Linux.
   - ✅ Build local Linux validado em WSL com OpenCV 4.14.0, ONNX Runtime 1.30.0, ABI 1 e Secure Core 0.2.0.
   - ✅ ABI C v1 pequena e versionada.
   - ✅ Secure Core v0.2.0.
   - ✅ Contexto persistente carrega YuNet, SFace e MiniFASNet uma única vez.
   - ✅ Build Linux dedicado em `scripts/build-biometric-core.sh`.
   - ✅ Manifesto local contém versão, ABI e SHA-256 da biblioteca.
   - ✅ Go usa `dlopen`/`dlsym`; nenhum spawn de CLI por captura.
   - ✅ Integração Go → `.so` validada no startup real: ABI/modelos carregados sem fallback.
   - ✅ Execução biométrica real validada pelo caminho Go → `.so`.

5. ⏳ **Unificar pipeline biométrico no Secure Core**
   - ✅ Orquestração multi-frame portada para C++.
   - ✅ YuNet → quality → PAD → SFace → embeddings → fusion → scoring no Secure Core.
   - ✅ Seleção dos 3 melhores frames próximos e embedding combinado no C++.
   - ⏳ Validar teste de paridade multi-frame da `.so` contra o baseline Python.

6. ⏳ **Retirar Python do runtime**
   - ✅ Secure Core C++ é o runtime padrão do `run-dev.sh`.
   - ✅ `--python-engine` virou fallback explícito de desenvolvimento/R&D.
   - ✅ Em modo nativo, não são criados/atualizados venv nem instalados NumPy/OpenCV Python/ONNX Runtime Python.
   - ✅ Em modo nativo, `engine_server.py` não é iniciado e a porta 8090 não é reservada.
   - ✅ Endpoints biométricos legados dependentes do Python ficam desabilitados no modo de produção.
   - ⏳ Validar startup e uma biometria completa sem processo Python.
   - Python permanece apenas em testes, validação e R&D.

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
