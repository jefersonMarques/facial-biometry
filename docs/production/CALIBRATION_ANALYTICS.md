# FaceProof — calibração observacional (primeira versão)

**Estado:** implementado na branch `feature/liveness-core-cpp`. Não constitui calibração certificada, homologação biométrica nem autorização para mudar limiares.

## Objetivo

Acumular evidências de verificações reais CNH + Face sem alterar o Secure Core C++, a validação criptográfica, o PAD, o liveness nem a decisão operacional. A coleta aproveita os eventos já emitidos:

- `cnh_tri_portrait_shadow`: PDF × VIO, PDF × captura, VIO × captura;
- `cnh_low_resolution_shadow`: bicúbica atual, nearest, bilinear e Lanczos4;
- `identity_completed`: conclusão da verificação, score e métricas de motor.

**Ativação de coleta:** `FACEPROOF_CNH_DUAL_PORTRAIT_SHADOW=true`. Continua desligada por padrão. A análise é independente e não bloqueia o processo.

## Interface

O console administrativo existente mostra a seção **Calibração biométrica**:

- períodos de 7, 30 ou 90 dias;
- filtro pelo `runtimeId` do SDK, e seleção de PDF × VIO, VIO × Face ou PDF × Face;
- número de CNHs concluídas e com evento de benchmark;
- decisões operacionais: aprovadas, rejeitadas e inconclusivas;
- quantidade de verificações sem identidade independentemente confirmada, titulares confirmados e impostores confirmados;
- histogramas de **20 intervalos** em scores cosseno entre −1 e +1, média e número de amostras;
- falhas de extração, tempo médio adicional por interpolação, qualidade média, liveness e PAD médios.

O endpoint `GET /v1/admin/calibration?days=30&runtime=...` exige **a mesma autenticação administrativa** usada pelo console. Retorna apenas agregados estatísticos: nenhuma foto, embedding, CPF, nome, ID da verificação, hash de sujeito ou eventos individuais.

Os histogramas PDF × Face são contados **uma única vez** por verificação, independentemente do número de interpolações VIO. Apenas o evento shadow mais recente de cada verificação é considerado.

## Rotulagem independente (ground truth)

A decisão do motor **não determina a classe real**:

| Campo `truth_label` | Interpretação |
| --- | --- |
| `unknown` | Não existe confirmação independente; valor padrão |
| `genuine_confirmed` | Titular confirmado por processo independente |
| `impostor_confirmed` | Impostor confirmado por processo independente |

A rotulagem opcional exige a seleção de uma categoria de origem:
`controlled_test`, `manual_review` ou `external_verified`. O administrador usa os controles nos **detalhes da verificação CNH**; a API autenticada é `PATCH /v1/admin/checks/{id}/truth` com `{"label":"impostor_confirmed","source":"controlled_test"}`. Retornar a `unknown` exige `source` vazio.

Cada alteração grava, **na mesma transação PostgreSQL**, o valor anterior e o novo em `admin_ground_truth_updated`. O registro não inclui fotos, CPF ou documentos. A autenticação atual identifica apenas a credencial administrativa; uma auditoria com identidade individual do analista é melhoria futura.

O campo preexistente `scenario` (por exemplo `genuine_live`) e `expectedDecision` **não são prova independente** e nunca são automaticamente promovidos para `truth_label`. Registros históricos não passam a ser rotulados sem revisão.

## Estrutura de persistência

O PostgreSQL já armazena scores e qualidade na tabela `faceproof_events` e os estados na tabela `faceproof_checks`. A migração aditiva adiciona:

- `truth_label TEXT NOT NULL DEFAULT 'unknown'`;
- `truth_source TEXT NOT NULL DEFAULT ''`;
- `truth_updated_at TIMESTAMPTZ`;
- índices para intervalo de conclusões CNH e eventos de benchmark.

Não há tabela nova de imagens, embeddings nem alteração de referência biométrica. Os agregados são calculados sob demanda diretamente no PostgreSQL. Os grupos são limitados à janela de até 90 dias, sem paginação de verificações pessoais para o navegador.

**Privacidade:** scores cosseno e classes de verdade, ainda que pseudonimizados, são informações sensíveis no contexto biométrico. Antes de operar continuamente é necessário registrar finalidade, base legal, política de retenção/exclusão, controle de acesso, medidas de segurança e informação aos titulares conforme LGPD. Nenhuma métrica deve ser enviada a terceiros sem fundamento e controles apropriados.

## Interpretação

- Scores não são probabilidade de identidade.
- Grupos pequenos são descritivos; 100, 500 ou 1.000 leituras não garantem significância nem desempenho de segurança.
- Verificações com `truth_label=unknown` ajudam a entender comportamento e desempenho, mas **não** permitem calcular FAR/FNMR.
- Grupos manualmente rotulados podem ter viés de seleção; também não equivalem automaticamente a um protocolo formal de validação.
- Nenhuma curva ROC, FNMR/FMR ou recomendação de limiar será emitida enquanto não houver conjunto genuíno/impostor suficiente, independente e representativo.
- `runtimeId` identifica o pacote SDK/browser; **não prova sozinho a versão/hash exatos de SFace, YuNet ou Secure Core**. Não comparar grupos de modelos de servidor distintos sem registrar essa proveniência. Completar versionamento do motor nativo é tarefa pré-produção.
- A qualidade calculada pelo motor não representa quantidade de pixels reais do VIO (45 × 61 no exemplo).
- O tempo bicúbico igual a zero é reutilização de trabalho anterior, **não latência total nula**.

## Prontidão para produção

1. Validar a migração e endpoint no banco do ambiente com backup e política de rollback.
2. Confirmar que a coleta shadow é amostrável/desligável e não compromete o tempo de resposta sob carga.
3. Revisar telemetria, retenção de eventos/checks e a rotina de expiração de embeddings temporários.
4. Definir controle de acesso administrativo por pessoa, política de auditoria e minimização de dados.
5. Capturar a proveniência dos modelos nativos para impedir agrupamentos estatísticos incorretos.
6. Executar testes controlados de impostores/PAD além da coleta natural de produção.

A tela e os agregados são o **primeiro instrumento de observação**, não substituem as medidas de preparação para produção.
