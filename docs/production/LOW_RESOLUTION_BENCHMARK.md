# Low-Resolution Portrait Benchmark (offline C++/OpenCV)

Status: **⏳ ferramenta de diagnóstico implementada; corpus de CNHs reais e validação estatística pendentes**.

## Objetivo

Medir, sem alterar o fluxo de decisão do FaceProof, o impacto da ampliação
da foto VIO de baixa resolução (um teste observou **45 × 61 px**) sobre
a extração YuNet/SFace e três pares de similaridade facial.

O benchmark **não** é usado pelo servidor, pela API nem pelo Web SDK.
Não altera modelo, Secure Core de produção, ABI, limiar biométrico ou decisão.
O CLI não exporta fotografias, caminhos de origem nem embeddings no JSON.

## Implementação

- `faceproof-biometric-core/tools/low_resolution_benchmark_cli.cpp`;
- `scripts/run-low-resolution-benchmark.py`: orquestra corpus local em CSV
  e agrupa métricas por rótulo (não exporta o CSV ou as fotografias);
- opcional com OpenCV compilado: executável
  `faceproof_low_resolution_benchmark`.

O benchmark carrega os **mesmos modelos YuNet e SFace** usados no caminho
de referência C++ e reaproveita `fp_bio_vision_reference_jpeg()`.

Métodos VIO:

1. `production_cubic`: imagem original entregue ao pipeline existente;
   internamente, `prepare_reference_image()` amplia com `INTER_CUBIC`.
2. `nearest`: ampliação explícita até as dimensões-alvo, com
   `INTER_NEAREST`, seguida pelo mesmo pipeline de referência.
3. `linear`: equivalente com `INTER_LINEAR`.
4. `lanczos4`: equivalente com `INTER_LANCZOS4`.

Para cada método, o JSON informa resolução original e preparada, fator
de escala em X/Y, dimensões estimadas do rosto na imagem original,
qualidade da referência VIO e scores **combinedCosine** e **robustCosine**
para `pdfVio`, `vioLive` e `pdfLive`. Uma falha de detecção recebe
`status=unavailable`; nunca gera score sintético de aprovação.

**Limite metodológico importante:** o terceiro arquivo é uma fotografia
da pessoa ao vivo, avaliada como **referência de imagem única** (com três
variantes de iluminação), não os 3 melhores frames próximos selecionados
pelo fluxo biométrico real. Os resultados de `pdfLive` neste utilitário
podem ser diferentes daqueles registrados no `biometric_match_computed`.
Não comparar os dois valores como se fossem a mesma medição.

A dimensão de rosto no VIO original é **estimada** dividindo a bounding box
detectada depois da ampliação pelo fator de escala; não é uma detecção
direta nos 45 × 61 px originais.

## Execução isolada em WSL/Linux

Usar o OpenCV pinado já instalado pelo projeto, sem recompilar o
`libfaceproof_core.so` nem modificar o serviço de identidade:

```bash
cd /mnt/d/wsl-home/jeferson/faceproof
git pull --ff-only origin feature/liveness-core-cpp

cmake -S faceproof-biometric-core -B build/low-resolution-benchmark -GNinja \
  -DCMAKE_BUILD_TYPE=Release \
  -DFACEPROOF_BIOMETRIC_WITH_OPENCV=ON \
  -DFACEPROOF_BIOMETRIC_BUILD_CLI=ON \
  -DOpenCV_DIR="$PWD/build/opencv-4.14.0/install/lib/cmake/opencv4"

cmake --build build/low-resolution-benchmark --target faceproof_low_resolution_benchmark
export LD_LIBRARY_PATH="$PWD/build/opencv-4.14.0/install/lib:${LD_LIBRARY_PATH:-}"

build/low-resolution-benchmark/faceproof_low_resolution_benchmark \
  /private/sample/pdf-portrait.png \
  /private/sample/vio-45x61.png \
  /private/sample/live-photo.jpg \
  models/yunet/face_detection_yunet_2023mar.onnx \
  models/sface/face_recognition_sface_2021dec.onnx
```

O exemplo exige imagens **extraídas e armazenadas localmente sob autorização**.
Não fornecer imagem completa da CNH, CPF, nome nem dados identificáveis
no nome de arquivo ou no relatório. O decoder BPG pode ser usado
isoladamente antes do benchmark, sem persistência adicional na API.

## Frontend local para três fotografias

A página de apoio para desenvolvimento está em
`services/api/cmd/lowresolutionui/index.html` e não faz parte da
interface pública de verificação ou do dashboard comercial. Serve apenas
para preparar o diagnóstico com amostras autorizadas sem enviar fotos para
servidores externos.

Após atualizar a branch e instalar o OpenCV fixado pelo projeto:

```bash
cd /mnt/d/wsl-home/jeferson/faceproof
git pull --ff-only origin feature/liveness-core-cpp
bash scripts/run-low-resolution-ui.sh
```

O script compila o CLI de benchmark, se necessário, sem recompilar ou
substituir a biblioteca Secure Core de produção. Abra no navegador:

`http://127.0.0.1:8765`

A interface recebe **três retratos já extraídos**, PNG ou JPEG:
foto original do PDF, fotografia original do QR/VIO decodificada com
`bpgdec` e uma foto facial ao vivo autorizada. Não aceita PDF inteiro
nem BPG bruto. Para preservar a comparação de baixa resolução,
**não aumente nem altere previamente a imagem VIO de 45 × 61 pixels**.

O frontend disponibiliza o JSON com scores, resolução, fator de ampliação,
localização estimada do rosto e quatro alternativas de interpolação;
pode copiar esse JSON para análise sem enviar retratos. A interface
só atende em `127.0.0.1`, limita arquivos a 4 MiB, usa token local e
verificação de origem; imagens são armazenadas temporariamente em diretório
privado durante a chamada nativa e removidas ao final do processamento
normal. Em encerramento forçado, arquivos temporários podem permanecer;
inspecione `/tmp/faceproof-private-benchmark-*` conforme a política
de segurança local. O parser OpenCV recebe imagens não confiáveis;
execute com usuário sem privilégios e não publique a porta na internet.

A ferramenta faz comparação com **uma fotografia de captura**. Não
substitui a comparação biométrica multiframes do serviço FaceProof.

## Corpus local e resultados agregados

Arquivo CSV privado (não versionar, não anexar ao repositório):

```csv
label,pdf_path,vio_path,live_path
genuine_live,pdf-001.png,vio-001.png,live-001.jpg
impostor_live,pdf-001.png,vio-001.png,live-outra-pessoa.jpg
```

**`impostor_live` significa que a fotografia ao vivo é de outra pessoa.**
As fotografias PDF e VIO devem continuar vindo da mesma CNH válida.
Nenhum documento adulterado deve ser criado para produzir a classe
impostora.

```bash
python3 scripts/run-low-resolution-benchmark.py \
  --manifest /private/dataset/manifest.csv \
  --binary "$PWD/build/low-resolution-benchmark/faceproof_low_resolution_benchmark" \
  --yunet "$PWD/models/yunet/face_detection_yunet_2023mar.onnx" \
  --sface "$PWD/models/sface/face_recognition_sface_2021dec.onnx" \
  --output /private/results/low-resolution.jsonl
```

O utilitário gera `low-resolution.jsonl` com scores por execução e
`low-resolution.summary.json` com contagem, mediana, mínimo e máximo
por classe, método e par.

O resultado não contém fotos ou embeddings. Entretanto, scores
biométricos vinculáveis a amostras também exigem proteção e controle
de acesso conforme política de retenção e LGPD. Recomenda-se
armazenamento criptografado fora do repositório e exclusão
após a análise autorizada.

## Interpretação e critérios para promoção

- Variantes interpoladas **não recuperam informação perdida**.
- O score SFace é cosseno bruto, **não probabilidade** de mesma identidade.
- `quality.score` do Secure Core não incorpora diretamente o número
  de pixels originais; `faceSize=1` não indica alta definição.
- `sharpness=0` e notas parecidas entre PDF e VIO não provam qualidade
  equivalente.
- Comparar separadamente casos genuínos e impostores, e por emissor,
  dispositivo e resolução quando houver dados suficientes.
- Não escolher automaticamente a maior similaridade: medir também
  a mudança de scores de impostores para evitar elevar falsos aceites.
- Não promover alteração de interpolação, alinhamento, super-resolução
  ou limiares ao runtime sem corpus rotulado, benchmark reprodutível,
  teste de ataque e aprovação específica.
- O benchmark não constitui certificação PAD nem prova independente de
  identidade. A análise continua desligada do caminho decisório.

## Estado atual

- ✅ CLI offline isolado e rotas de comparação adicionados ao código.
- ⏳ Testar com CNHs legitimamente obtidas e amostras de impostores autorizadas.
- ⏳ Validar determinismo, confiança por grupo e latência em WSL real.
- ⏳ Comparar original vs pré-processamentos em corpus representativo.
- ⬜ Alterar o runtime de produção: **fora do escopo desta etapa**.
