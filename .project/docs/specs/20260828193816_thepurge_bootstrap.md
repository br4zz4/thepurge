# The Purge — Spec (bootstrap v1)

Você é um Engenheiro de Software Sênior especialista em ferramentas de sistema para macOS e desenvolvimento de CLIs de alta performance usando Go (Golang) ou Rust.

Crie uma ferramenta de sistema chamada "The Purge". O projeto consiste em duas partes: um Daemon (serviço de segundo plano) que monitora o uso do SSD e aplica políticas automáticas de limpeza, e um CLI interativo e colorido para gerenciamento manual em lote (batch).

O software deve ser projetado com base em aprendizados reais do comportamento do macOS (permissões TCC, comportamento de arquivos ocultos e estruturas de caches de aplicativos modernos).

> **Premissa de permissões corrigida:** quem controla leitura/escrita em `~/Library/*`, `~/Library/Containers/*` e demais pastas do usuário é o **TCC** (Privacy & Security), não o SIP. O SIP protege o volume do sistema (`/System`, `/usr`, `/bin`) e não é o obstáculo desta ferramenta. Sem **Full Disk Access (FDA)** concedido manualmente (System Settings → Privacy & Security → Full Disk Access), o daemon não consegue nem **ler** as pastas-alvo — isso não é falha de permissão tratável com "log WARN e segue", é bloqueio total. O passo do FDA faz parte da instalação (seção launchd).

**REGRAS DE SEGURANÇA (invioláveis):**
1. Só deleta o que o config autoriza: caches e logs em `eviction_targets` e pastas em `enabled_orphan_apps` (órfãos habilitados = consentimento explícito via CLI).
2. Nunca deleta pastas de apps instalados ou rodando sem consentimento explícito do usuário.
3. Deleta o mais antigo/pesado primeiro, para liberar espaço com risco mínimo.
4. Se o espaço NÃO voltar após eviction, NÃO deleta mais em massa sem autorização explícita do usuário.
5. Nunca deleta arquivos immutable/sflags (`UF_IMMUTABLE`, `SF_IMMUTABLE`, `UF_APPEND` via `syscall.Stat_t.st_flags`, `chflags`, xattr `com.apple.*`) — pode quebrar o disco ou o app. Pular + log WARN.

---

### 1. ARQUIVO DE CONFIGURAÇÃO (config.yml)

Gere o layout padrão do arquivo que ficará em `~/.config/thepurge/config.yml`. Ele deve suportar limites por tamanho absoluto ou porcentagem. **O exemplo abaixo deve passar num parser YAML — ele é o fixture de teste do parser do config.**

```yaml
storage:
  # Volume-alvo do monitoramento (default: raiz do volume de dados)
  volume: "/"
  # Aceita "percent" ou "size"
  threshold_type: "percent"
  # Se for percent: 15 (significa 15% livre). Se for size: "30GB" (aceita KB/MB/GB/TB)
  threshold_value: 15
  # Margem de folga: para de evictar ao atingir threshold + margem (ex.: 15% + 5% = 20% livre)
  safety_margin_percent: 5
  # Frequência do scan caro (eviction) do daemon
  check_interval_minutes: 30
  # Frequência da checagem barata de espaço (statvfs) — pilar do gatilho de emergência
  poll_interval_seconds: 60
  # Teto de deleção por ciclo (em bytes por run) — cumpre a regra 4 sem depender só do espaço
  max_bytes_per_run: "10GB"

# Escopo V1: deleta SÓ caches, logs e órfãos habilitados. Sem backup/snapshot/undo em V1 (pode voltar depois).
scope_v1:
  delete_types: ["caches", "logs"]
  # Prioridade de deleção: logs primeiro (liberam espaço rápido), depois caches.
  priority_types: ["logs", "caches"]

# V1: o daemon respeita apps rodando (não deleta conteúdo de apps vivos)
protection:
  respect_running_apps: true

# Proteção contra arquivos immutable/sflags (pular + log WARN, não deletar — pode quebrar o disco ou o app)
immutable:
  check_immutable: true

# Log persistente e rotacionado + alerta ao usuário quando a eviction falhar em liberar espaço
logging:
  log_path: "~/Library/Logs/thepurge/thepurge.log"
  notify_on_failure: true

eviction_targets:
  # Cada alvo tem `type`: "cache" ou "log", usado por delete_type/priority_types.
  # `subfolders` ou `pattern` selecionam o conteúdo; nunca o path raiz do alvo.
  safe_caches:
    - path: "~/Library/Application Support/Claude"
      type: cache
      subfolders: ["Cache", "Code Cache", "GPUCache"]
      # NOTA: "Local Storage"/"IndexedDB" NÃO entram — é estado de usuário (sessão, histórico), não cache.
    - path: "~/Library/Application Support/Arc"
      type: cache
      pattern: "*Cache*"
    - path: "~/Library/Application Support/Google/Chrome"
      type: cache
      pattern: "*Cache*"
    - path: "~/Library/Application Support/stremio-server"
      type: cache
      subfolders: ["stremio-cache"]
    - path: "~/Library/Containers/com.tinyspeck.slackmacgap"
      type: cache
      subfolders: ["Data/Library/Caches", "Data/Library/Application Support/Slack/Cache"]
    - path: "~/Library/Containers/com.microsoft.teams2"
      type: cache
      subfolders: ["Data/Library/Caches", "Data/Library/Application Support/Microsoft/Teams/Cache"]
    - path: "~/Library/Application Support/com.apple.wallpaper"
      type: cache
      subfolders: ["Aerials"]
      # Nota: o sistema re-baixa os wallpapers; tratado como cache de download.

  # Alvos de LOG. (A spec anterior não definia nenhum — sem isso, delete_types/priority_types eram inimplementáveis.)
  logs:
    - path: "~/Library/Logs"
      type: log
      pattern: "*"
    - path: "~/Library/Containers/com.tinyspeck.slackmacgap/Data/Library/Logs"
      type: log
      pattern: "*"

  # Caches/log de apps que SÓ podem ser deletados quando o app NÃO está rodando (processo vivo).
  # Cada entrada pode declarar `bundle_id` e/ou `process_names` (opcional) para detecção confiável;
  # default: derivar do nome do path.
  volatile_apps:
    - path: "~/Library/Application Support/Arc"
      bundle_id: "company.thebrowser.Browser"
    - path: "~/Library/Application Support/Google/Chrome"
      bundle_id: "com.google.Chrome"
    - path: "~/Library/Application Support/stremio-server"
      bundle_id: "com.stremio.stremio"
    - path: "~/Library/Application Support/Microsoft Edge"
      bundle_id: "com.microsoft.edgemac"
    - path: "~/Library/Application Support/Microsoft Office"
      bundle_id: "com.microsoft.office"

  # Pastas órfãs de apps desinstalados. Por default INATIVAS (proteção contra falso positivo).
  # "Desinstalado" é determinado pelo daemon (ver seção 2), não só pela palavra do usuário.
  orphan_apps:
    - "~/Library/Application Support/Cursor"
    - "~/Library/Application Support/Spotify"
    - "~/Library/Application Support/discord"
    - "~/Library/Application Support/Notion Calendar"
    - "~/Library/Application Support/com.imobie.AnyTrans"
    - "~/Library/Application Support/com.docker.install"

  # Órfãos que o daemon PODE deletar integralmente — enable via `thepurge config orphans.enable --path "<PATH>"`
  enabled_orphan_apps: []

retention:
  # Peso de priorização por idade (mtime) na ordenação; NÃO é piso duro.
  # Em emergência (espaço livre < 5%), o daemon pode ignorar.
  min_age_days: 7
```

---

### 2. REQUISITOS DO DAEMON (MONITORAMENTO EM SEGUNDO PLANO)

- **Gatilho de Emergência:** o daemon roda silenciosamente e faz uma **checagem barata de espaço** (`statvfs`/`f_bavail`) a cada `poll_interval_seconds` (ex.: 60s). Se o espaço livre violar o threshold (ex.: abaixo de 15% ou 30GB), ele dispara a **Estratégia de Evicção** — um scan caro, limitado por `check_interval_minutes` entre execuções quando não há emergência.
  - **Métrica de espaço:** `statfs.f_bavail` (blocos disponíveis a não-root) no volume definido em `storage.volume`. Nota: o Finder conta espaço *purgeable* do APFS como "livre", mas purgeable não é espaço devolvido numa eviction — reportar sempre `f_bavail` real.
- **Estratégia de Evicção Inteligente (V1 = caches e logs):**
  1. Primeiro, remove completamente as pastas habilitadas em `enabled_orphan_apps` (se existirem no disco e passarem na detecção de órfão abaixo).
  2. Se não for suficiente, varre `safe_caches` e `logs`, ordena tudo por `mtime` e `ctime` (NÃO `atime` — o APFS atualiza atime de forma lazy e não-confiável) e pelo tamanho, e deleta os mais antigos/pesados primeiro. **Prioriza logs** (liberam espaço rápido) antes dos caches, quando possível (via `priority_types`).
  3. Para de deletar assim que o espaço livre atingir `threshold + safety_margin_percent` (ex.: limite 15%, limpa até 20% livre) **ou** quando atingir `max_bytes_per_run` neste ciclo, o que vier primeiro.
  4. Re-checa o espaço com tolerância: `unlink` de arquivo ainda aberto (ex.: cache de app recém-fechado) não libera espaço imediatamente no APFS. Se o espaço "não voltou", espera o próximo ciclo (backoff) antes de concluir falso negativo da regra 4.

- **🛡️ Regras de Segurança (safety rules):**
  - **Respeitar apps rodando:** antes de deletar conteúdo de um app, checar se está rodando via `bundle_id`/`process_names` do alvo (fallback: nome derivado do path). **Re-checar antes de cada deleção** — o usuário pode abrir o app durante o run. `volatile_apps` sempre pulam se rodando; os demais pulam se `respect_running_apps: true`. Log WARN. `--force` só existe no CLI (seção 3).
  - **Detectar items immutable/sflags:** checar `syscall.Stat_t.st_flags` (`UF_IMMUTABLE`, `SF_IMMUTABLE`, `UF_APPEND`), `chflags` e xattr `com.apple.*`. Se immutable, pular e log WARN — não deletar.
  - **Permissões/TCC:** se houver falha de permissão em algum item, log WARN, pular e continuar. Exceção: se **nenhum** alvo for legível (indício de falta de FDA), log de erro + notificação — não é caso de "continuar silenciosamente".
  - **Never logs secrets/credentials:** nunca logar secrets, credenciais ou conteúdo sensível no log.
- **Detecção de órfãos (antes de deletar pasta inteira):** o app não existe em `/Applications`, `/Applications/Utilities`, `~/Applications`, **e** nenhum processo correspondente está ativo. A regra "respect running apps" vale também para órfãos (reinstalado em outro local ≠ desinstalado).
- **Hot-reload:** o daemon re-lê o config a cada ciclo (o efeito de `thepurge config orphans.enable` vale a partir do próximo tick); `SIGHUP` força reload imediato.
- **Alerta ao usuário:** log persistente e rotacionado (`logging.log_path`); notificação via `osascript` quando a eviction falhar em liberar espaço (regra 4) ou quando houver indício de falta de FDA/TCC.

---

### 3. CLI INTERATIVO (gerenciamento manual em lote)

O CLI é metade do produto e o único caminho para ações *force* e edição de config:

- `thepurge status` — espaço livre real (`f_bavail`), threshold, config carregada, último run do daemon (timestamp, bytes libertados, erros).
- `thepurge purge` — eviction manual em lote. **`--dry-run` é o default** (lista o que seria deletado, com tamanho e idade); `--apply` executa de verdade; `--force` permite tocar conteúdo de apps rodando — **exige `--dry-run` executado antes** (segurança da regra 2).
- `thepurge config get|set|validate` — leitura/edição do YAML; `validate` checa parser YAML, threshold (0–100 para percent, sufixo válido para size) e existência dos paths.
- `thepurge config orphans.enable --path "<PATH>"` / `config orphans.disable --path "<PATH>"` — Opt-in explícito de órfãos (alimenta `enabled_orphan_apps`).
- `thepurge daemon install|uninstall|status` — instala/remove o LaunchAgent e mostra estado.
- `thepurge doctor` — pré-flight: valida FDA (tenta listar um path protegido por TCC), parse do config, espaço livre, paths-alvo existentes.
- **TUI colorido** (Bubble Tea/Lipgloss) sobre `purge --dry-run`: lista alvos com tamanho/idade, seleção múltipla, e confirmação visual antes do `--apply`.

---

### 4. INICIALIZAÇÃO NATIVA (launchd)

- **LaunchAgent de usuário** (NÃO LaunchDaemon/root — como root os arquivos criados ficam root-owned e o TCC não se aplica ao contexto do agente): `~/Library/LaunchAgents/br4zz4.thepurge.plist`.
- Chaves: `RunAtLoad`, `KeepAlive`, `ThrottleInterval`, `ProcessType: Background`, `StandardOutPath`/`StandardErrorPath` apontando para o log (`~/Library/Logs/thepurge/`).
- Instalação: `launchctl bootstrap gui/$UID ~/Library/LaunchAgents/br4zz4.thepurge.plist` (e `bootout` para remover).
- **Ordem obrigatória na instalação:** (1) `thepurge doctor` valida FDA → (2) usuário concede Full Disk Access em System Settings → (3) `thepurge daemon install`. Instalar sem FDA = daemon roda e não deleta nada.

---

### 5. DECISÃO DE LINGUAGEM (V1 = Go)

**Use Go (Golang).** Razões: (1) builds nativos limpos no macOS — sem dor do linker C do Rust; (2) stdlib cobre as APIs de sistema que a tool precisa (stat/xattr com `syscall.Stat_t`, strings, exec, goroutines) sem CGo; (3) TUI com Bubble Tea/Lipgloss. Cautela: o config é YAML → precisa de 1 dep (`gopkg.in/yaml.v3`, pequeno e estável); FSEvents é contornado por polling em dois níveis (barato de espaço a cada 60s + scan só na emergência). A alternativa Rust seria mais rápida (sem GC), mas só importa em alta frequência — aqui o daemon acorda a cada 30 min, então o Go ganha no brilho de build e menos deps.

### ENTREGÁVEIS
1. Arquitetura de arquivos do projeto.
2. Código-fonte completo documentado (sem pseudocódigo).
3. Instruções de compilação para Go (cross-compile para `darwin/arm64` com `GOOS=darwin GOARCH=arm64`) e o script/configuração de inicialização nativa para macOS usando `launchd` (`.plist`), incluindo o passo do Full Disk Access, para que o daemon sobreviva ao reiniciar o Mac.
4. Teste de aceitação do config: o YAML de exemplo desta spec deve passar no parser (fixture de teste) — nada de `storage:` duplicado ou chaves repetidas.