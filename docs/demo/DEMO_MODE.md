# FaceProof Demo Mode

Estado: **em validação**

O modo de demonstração usa o mesmo FaceProof Secure Core C++ do runtime normal.
Não existe motor biométrico alternativo ou resultado simulado.

## Experiências

1. ⏳ **CNH + Face**
   - CNH Digital autenticada.
   - Foto oficial como referência.
   - Prova de vida + comparação 1:1.
   - Em modo demo, referência e melhor captura podem ser exibidas no console autenticado.

2. ⏳ **Cadastro facial + revalidação**
   - Cadastro: prova de vida cria um template biométrico criptografado.
   - Revalidação: novo link usa o template cadastrado como referência 1:1.
   - O mesmo Secure Core C++ executa os dois passos.

3. ⏳ **Foto de referência**
   - Uma foto é cadastrada no console.
   - O Secure Core extrai a referência facial.
   - O link público executa prova de vida e comparação 1:1.
   - Demonstra integração com onboarding, cadastro interno, crachá ou outra origem de referência.

## Console

⏳ **Login**
- Usuário e senha de demonstração.
- HTTP Basic somente no gateway admin autenticado.
- O Bearer administrativo existente continua compatível.

⏳ **Gerador de links**
- Seleção visual entre as três experiências.
- Cadastro/revalidação compartilham o mesmo grupo de experiência.
- Validade configurável.
- URL pública opcional para tunnel/ambiente externo.

⏳ **Histórico visual**
- Lista das biometrias recentes.
- Foto de referência e melhor captura lado a lado.
- Resultado, score facial, liveness e PAD.
- Modal técnico sob demanda.

## Privacidade

O modo demo é explícito em `FACEPROOF_DEMO_MODE=true`.

Quando habilitado, a foto de referência e a melhor captura podem ser mantidas junto ao
check/template dentro dos repositórios criptografados existentes. Essas imagens:

- não são gravadas no PostgreSQL analytics;
- não são incluídas em relatórios técnicos exportados;
- só são retornadas por endpoints administrativos autenticados;
- não alteram o motor biométrico nem a decisão;
- devem ser desabilitadas fora de demonstrações controladas.

O histórico visual do console busca no máximo 60 checks por vez para evitar payloads excessivos.

## Interface

Paleta Rododata:

- verde: `#b7ff32`
- preto: `#10130f`
- fundo: `#f5f8ef`

A experiência pública é mobile-first. Em notebook, o conteúdo permanece estreito,
centralizado e semelhante ao celular, apenas com largura maior.

## Validação pendente

Antes de marcar os itens acima como concluídos:

```bash
cd ~/faceproof/services/api
go test ./...
go build ./cmd/server
go build ./cmd/devgateway

cd ~/faceproof/apps/web-sdk
npm run typecheck
npm run build
npm run test:liveness-v2

cd ~/faceproof
node --check apps/admin/app.js
```

Depois validar em runtime:

- login no console;
- geração CNH;
- cadastro facial;
- revalidação do mesmo cadastro;
- validação por foto de referência;
- referência ↔ captura no histórico.
