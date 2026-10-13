# FaceProof Product Structure

Estado: **estrutura comercial em construção**

O FaceProof deve ser familiar para empresas que já integram plataformas biométricas, sem depender de implementação proprietária de terceiros.

## 1. FaceProof Server

Responsável pela autoridade da verificação.

```text
Go API
  ↓
FaceProof Secure Core C++
  ↓
YuNet + SFace + MiniFASNet + liveness
  ↓
template / decisão / auditoria
```

Inclui:

- API de criação e consulta de verificações;
- sessões e Capture Protocol;
- templates biométricos;
- autenticação por tenant;
- quota;
- Model Pack;
- auditoria e analytics;
- armazenamento criptografado.

## 2. FaceProof Web SDK

Responsável pela experiência de captura no navegador.

Inclui:

- câmera;
- guia de enquadramento;
- longe → perto;
- MediaPipe;
- WASM;
- qualidade local;
- Capture Seal;
- marca visual dinâmica;
- envio dos frames;
- tratamento de retry/idempotência.

Regra arquitetural:

> O browser filtra e coleta. O servidor valida e decide.

## 3. FaceProof Console

Painel de operação e integração.

### Já existente no modo demo

- ⏳ login privado;
- ⏳ geração de links;
- ⏳ listagem visual das verificações;
- ⏳ referência × captura;
- ⏳ resultado, liveness, PAD e similaridade;
- ⏳ detalhes técnicos;
- ⏳ três famílias de fluxo.

Os itens permanecem `⏳` até validação runtime da versão atual.

### Console de produção

Planejado:

- API keys por tenant;
- rotação/revogação de API key;
- domínios/origens permitidos;
- configuração de callbacks/webhooks;
- políticas de retenção;
- limites/quota;
- configuração de fluxos;
- histórico e filtros;
- detalhe da biometria;
- exportação/auditoria;
- uso mensal;
- configuração de SDK.

## 4. Exemplos de integração

O pacote de referência terá experiências simples e separadas.

### A. Cadastro facial → comparação biométrica

```text
Cadastro
pessoa + liveness
      ↓
template biométrico

Revalidação
pessoa + liveness
      ↓
novo embedding
      ↓
template × biometria atual
```

### B. CNH → biometria

```text
CNH Digital
   ↓
foto oficial / embedding
   +
captura com liveness
   ↓
comparação 1:1
```

### C. CNH preparada pelo painel → biometria

Fluxo alvo:

```text
Operador / API
envia ou autentica a CNH
        ↓
FaceProof prepara referência
        ↓
link público
        ↓
usuário faz somente a biometria
```

Este terceiro fluxo ainda precisa ser promovido do modelo de demonstração para um contrato de API/console de produção.

## 5. FaceProof Mobile SDK

Próxima camada do produto.

Objetivos:

- Android primeiro;
- Secure Core C++ nativo;
- templates protegidos no dispositivo;
- comparação 1:1 offline;
- liveness/PAD local;
- evidência offline assinada;
- sincronização posterior;
- Android Keystore / iOS Secure Enclave;
- attestation quando houver rede.

## Contrato conceitual para clientes

```text
                 ┌───────────────────────┐
                 │   FaceProof Console   │
                 │ API · config · audit  │
                 └───────────┬───────────┘
                             │
Cliente / ERP ───────────────┼──── FaceProof API
                             │
                    ┌────────▼────────┐
                    │ FaceProof Server│
                    │ + Secure Core   │
                    └────────┬────────┘
                             │
           ┌─────────────────┴─────────────────┐
           │                                   │
     FaceProof Web SDK                  FaceProof Mobile SDK
           │                              (roadmap)
           │
     usuário / câmera
```

A implementação biométrica crítica permanece privada. Clientes recebem contratos de API, SDKs e exemplos, não thresholds internos, pesos, heurísticas ou código do Secure Core.
