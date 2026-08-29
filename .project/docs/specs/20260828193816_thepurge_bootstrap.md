Você é um Engenheiro de Software Sênior especialista em ferramentas de sistema para macOS e desenvolvimento de CLIs de alta performance usando Go (Golang) ou Rust.

Crie uma ferramenta de sistema chamada "The Purge". O projeto consiste em duas partes: um Daemon (serviço de segundo plano) que monitora o uso do SSD e aplica políticas automáticas de limpeza, e um CLI interativo e colorido para gerenciamento manual em lote (batch).

O software deve ser projetado com base em aprendizados reais de comportamento do macOS (gerenciamento de permissões do SIP, comportamento de arquivos ocultos e estruturas de caches de aplicativos modernos).

**REGRAS DE SEGURANÇA (invioláveis, V1 = só caches e logs):** (1) Só deleta o que o config autoriza (caches e logs em `eviction_targets`). (2) Nunca deleta pastas de apps instalados ou rodando sem consentimento explícito do usuário. (3) Deleta o mais antigo/pesado primeiro, para liberar espaço com risco mínimo. (4) Se o espaço NÃO voltar após eviction, NÃO deleta mais em massa sem autorização explícita do usuário. (5) Nunca deleta arquivos RMS/immutable (pode quebrar o disco).

---

### 1. ARQUIVO DE CONFIGURAÇÃO (config.yml)
Gere o layout padrão do arquivo que ficará em `~/.config/thepurge/config.yml`. Ele deve suportar limites por tamanho absoluto ou porcentagem.

```yaml
storage:
  # Aceita "percent" ou "size"
  threshold_type: "percent"
  # Se for percent: 15 (significa 15% livre). Se for size: "30GB"
  threshold_value: 15
  # Frequência de checagem do daemon
  check_interval_minutes: 30

# Escopo V1: deleta SÓ caches e logs. Sem backup/snapshot/undo em V1 (pode voltar depois).
scope_v1:
  delete_types: ["caches", "logs"]
  # Prioridade de deleção: logs primeiro (liberam espaço rápido), depois caches.
  priority_types: ["logs", "caches"]

storage:
  # Aceita "percent" ou "size"
  threshold_type: "percent"
  # Se for percent: 15 (significa 15% livre). Se for size: "30GB"
  threshold_value: 15
  # Frequência de checagem do daemon
  check_interval_minutes: 30

# V1: o daemon respeita apps rodando (não deleta conteúdo de apps vivos)
protection:
  respect_running_apps: true

# Proteção contra RMS/immutable (pular + log WARN, não deletar — pode quebrar o disco ou o app)
immutable:
  check_ims: true

eviction_targets:
  # Caches conhecidos e seguros para apagar o conteúdo
  safe_caches:
    - path: "~/Library/Application Support/Claude"
      subfolders: ["Cache", "Code Cache", "Local Storage"]
    - path: "~/Library/Application Support/Arc"
      pattern: "*Cache*"
    - path: "~/Library/Application Support/Google/Chrome"
      pattern: "*Cache*"
    - path: "~/Library/Application Support/stremio-server"
      subfolders: ["stremio-cache"]
    - path: "~/Library/Containers/com.tinyspeck.slackmacgap"
      subfolders: ["Data/Library/Caches", "Data/Library/Application Support/Slack/Cache"]
    - path: "~/Library/Containers/com.microsoft.teams2"
      subfolders: ["Data/Library/Caches", "Data/Library/Application Support/Microsoft/Teams/Cache"]
    - path: "~/Library/Application Support/com.apple.wallpaper"
      subfolders: ["Aerials"]

  # Caches/log de apps que sò deletam quando o app NÃO está rodando (processo vivo)
  volatile_apps:
    - path: "~/Library/Application Support/Arc"
    - path: "~/Library/Application Support/Google/Chrome"
    - path: "~/Library/Application Support/stremio-server"
    - path: "~/Library/Application Support/Microsoft Edge"
    - path: "~/Library/Application Support/Microsoft Office"

  # Pastas órfãs de apps desinstalados. Por default INATIVAS (proteção contra falso positivo)
  orphan_apps:
    - "~/Library/Application Support/Cursor"
    - "~/Library/Application Support/Spotify"
    - "~/Library/Application Support/discord"
    - "~/Library/Application Support/Microsoft Edge"
    - "~/Library/Application Support/Notion Calendar"
    - "~/Library/Application Support/com.imobie.AnyTrans"
    - "~/Library/Application Support/com.docker.install"

  # Ophans que o daemon PODE deletar integralmente — enable via `thepurge config orphans.enable --path "<PATH>"`
  enabled_orphan_apps: []

retention:
  # Idade mínima (atime/mtime) para o daemon priorizar a deleção automática
  min_age_days: 7
```

---

### 2. REQUISITOS DO DAEMON (MONITORAMENTO EM SEGUNDO PLANO)
- **Gatilho de Emergência:** O daemon roda silenciosamente. Se o espaço livre violar o threshold (ex: cair abaixo de 15% ou 30GB), ele inicia a "Estratégia de Evicção".
- **Estratégia de Evicção Inteligente (V1 = caches e logs):**
  1. Primeiro, ele remove completamente as pastas habilitadas em `enabled_orphan_apps` (se existirem no disco).
  2. Se não for suficiente, ele varre `safe_caches` e pastas de logs, ordena tudo pelo timestamp de último acesso (`atime`) ou modificação (`mtime`) e pelo tamanho, e deleta os mais antigos/pesados primeiro. **Prioriza logs** (liberam espaço rápido) antes dos caches, quando possível.
  3. Parar de deletar assim que o espaço livre do Mac atingir o threshold de segurança + margem de folga de 5% (ex: se o limite era 15%, limpa até 20% livre).

- **🛡️ Regras de Segurança (safety rules):** (não importar jamais)
  - **Respeitar apps rodando (app running):** antes de deletar conteúdo de um app, checar se o app está rodando. Se estiver, pular (especialmente `volatile_apps`), a menos que o usuário force via flag `--force`. Log WARN.
  - **Detetar items RMS/immutable:** checar flags/attributes (RMS, hardlink, lock, metadata `.com.apple.*`). Se for RMS/immutable, pular e log WARN (não deletar — pode quebrar o disco ou o app).
  - **Permissões:** se houver falha de permissão em algum item, log WARN, pular e continuar sem travar.
  - **Never logs secrets/credentials:** nunca logar secrets, credenciais ou conteúdo sensível no log.

---

### 3. DECISÃO DE LINGUAGEM (V1 = Go)
**Use Go (Golang).** Razo: (1) builds nativos limpos no macOS — sem dor do linker C do Rust; (2) stdlib cobre as APIs de system que o tool precisa (stat/xattr com `syscall.Stat_t`, strings, exec, goroutines) sem CGo; (3) TUI com Bubble Tea/Lipgloss. Cautela: o config é YAML → precisa de 1 dep (`gopkg.in/yaml.v3`, pequeno e estável); FSEvents pode ser contornado por polling a cada `check_interval_minutes`. Alternativa Rust seria mais rápida (sem GC) mas só importante em alta frequência — aqui o daemon acorda a cada 30 min, então o Go ganha no brilho de build e menos deps.

### ENTREGÁVEIS
1. Arquitetura de arquivos do projeto.
2. Código-fonte completo documentado (sem pseudocódigo).
3. Instruções de compilação para Go (cross-compile para `darwin/arm64` com `GOOS=darwin GOARCH=arm64`) e o script/configuração de inicialização nativa para macOS usando `launchd` (`.plist`) para que o daemon sobreviva ao reiniciar o Mac.
