# The Purge — Spec (V1 completa)

> **Status:** Implementação V1 completa.  
> **Licença:** GPLv3  
> **Linguagem:** Go 1.27  
> **Build:** `GOOS=darwin GOARCH=arm64 go build -o thepurge ./cmd/thepurge`  
> **Dependências:** `golang.org/x/sys` (estatísticas de disco), `gopkg.in/yaml.v3` (config)

---

## 0. Premissa de permissões (corrigida)

Quem controla leitura/escrita em `~/Library/*`, `~/Library/Containers/*` e demais pastas do usuário é o **TCC** (Privacy & Security), não o SIP. O SIP protege o volume do sistema (`/System`, `/usr`, `/bin`) e não é o obstáculo desta ferramenta.

Sem **Full Disk Access (FDA)** concedido manualmente (System Settings → Privacy & Security → Full Disk Access), o daemon não consegue nem **ler** as pastas-alvo — isso não é falha de permissão tratável com "log WARN e segue", é bloqueio total. O passo do FDA faz parte da instalação (seção launchd).

---

## 1. REGRAS DE SEGURANÇA (invioláveis)

1. Só deleta o que o config autoriza: caches e logs em `eviction_targets` e pastas em `enabled_orphan_apps` (órfãos habilitados = consentimento explícito via CLI).
2. Nunca deleta pastas de apps instalados ou rodando sem consentimento explícito do usuário.
3. Deleta o mais antigo/pesado primeiro, para liberar espaço com risco mínimo.
4. Se o espaço NÃO voltar após eviction, NÃO deleta mais em massa sem autorização explícita do usuário.
5. Nunca deleta arquivos immutable/sflags — `UF_IMMUTABLE`, `SF_IMMUTABLE`, `UF_APPEND`, `SF_APPEND` via `syscall.Stat_t.st_flags`, `chflags`. Pular + log WARN.

---

## 2. ARQUIVO DE CONFIGURAÇÃO (`~/.config/thepurge/config.yml`)

Suporta limites por tamanho absoluto ou porcentagem. O YAML da spec é o fixture de teste (`config/testdata/config.yml`) e deve passar no parser sem duplicatas.

### Estruturas de dados (`config/config.go`)

| Campo | Tipo | Descrição |
|---|---|---|
| `storage.volume` | string | Volume-alvo (default: "/") |
| `storage.threshold_type` | string | "percent" ou "size" |
| `storage.threshold_value` | float64 | 15 (=15% livre) ou "30GB" (em bytes) |
| `storage.safety_margin_percent` | int | Para ao atingir threshold + margem |
| `storage.check_interval_minutes` | int | Frequência do scan caro (30 min) |
| `storage.poll_interval_seconds` | int | Frequência da checagem barata de espaço (60s) |
| `storage.max_bytes_per_run` | string | Teto de deleção por ciclo ("10GB") |
| `scope_v1.delete_types` | []string | ["caches", "logs"] |
| `scope_v1.priority_types` | []string | ["logs", "caches"] |
| `protection.respect_running_apps` | bool | true (default) |
| `immutable.check_immutable` | bool | true (default) |
| `logging.log_path` | string | "~/Library/Logs/thepurge/thepurge.log" |
| `logging.notify_on_failure` | bool | true |
| `eviction_targets.safe_caches` | []Target | 7 targets V1 |
| `eviction_targets.logs` | []Target | 2 targets V1 |
| `eviction_targets.volatile_apps` | []Target | 5 apps (opt-in) |
| `eviction_targets.orphan_apps` | []string | 6 pastas órfãs (default inativas) |
| `eviction_targets.enabled_orphan_apps` | []string | Opt-in via CLI |
| `retention.min_age_days` | int | 7 (peso de priorização, não piso duro) |

### Target

| Campo | Tipo | Descrição |
|---|---|---|
| `path` | string | Path do alvo (com `~/`) |
| `type` | string | "cache" ou "log" |
| `subfolders` | []string | Subpastas |
| `pattern` | string | Glob pattern |
| `bundle_id` | string | Bundle ID (running-apps check) |

### Default seguro (`config/default.go`)

- `volatile_apps`, `orphan_apps`, `enabled_orphan_apps`: vazios (opt-in)
- `respect_running_apps`: true, `check_immutable`: true
- `min_age_days`: 7

### Targets V1 (default shipping)

**Safe caches (7):**
- Claude → [Cache, Code Cache, GPUCache] (NOTA: "Local Storage"/"IndexedDB" NÃO entram)
- Arc → pattern "*Cache*"
- Chrome → pattern "*Cache*"
- Stremio → stremio-cache
- Slack → [Data/Library/Caches, Data/Library/Application Support/Slack/Cache]
- Teams → [Data/Library/Caches, Data/Library/Application Support/Microsoft/Teams/Cache]
- Wallpaper (com.apple.wallpaper) → Aerials

**Logs (2):**
- ~/Library/Logs → "*"
- ~/Library/Containers/com.tinyspeck.slackmacgap/Data/Library/Logs → "*"

**Volatile apps (5 — opt-in):** Arc, Chrome, Stremio, Edge, Microsoft Office

**Orphan apps (6 — opt-in):** Cursor, Spotify, Discord, Notion Calendar, AnyTrans, Docker

---

## 3. ARQUITETURA DE PACOTES

```
cmd/thepurge/main.go          → Ponto de entrada (15 loc, 1 func)
config/
  config.go                   → Load/Marshal YAML (88 loc, 2 funcs)
  default.go                  → Config default seguro (48 loc, 1 func)
  testdata/config.yml         → Fixture de teste do YAML
internal/
  cli/                        → CLI interativo (~800 loc total)
    run.go                    → Dispatcher + help
    status.go                 → thepurge status
    doctor.go                 → thepurge doctor
    purge.go                  → thepurge purge
    config.go                 → configPath + loadConfig
    configcmd.go              → config subcommands
    configfmt.go              → renderConfig
    yamlcolor.go              → ANSI colorização YAML
    logs.go                   → thepurge logs
    servicecmd.go             → service install/uninstall/status
    summary.go                → Digest dry-run
  eviction/                   → Engine de evicção (~340 loc)
    engine.go                 → Run() orquestrador
    scan.go                   → ScanTargets (walk + ordenação)
    delete.go                 → DeleteItems (safety checks)
    targets.go                → ResolveTargets (expansão config→items)
  fsutil/                     → Utilitários macOS (~125 loc)
    bytes.go                  → ParseBytes / FormatBytes
    space_darwin.go           → DiskUsage (f_bavail)
    immutable_darwin.go       → IsImmutable (chflags)
  logging/                    → Logger (~63 loc)
    logging.go                → Info/Warn/Error timestampado
  service/                    → launchd (~212 loc)
    install.go                → Install/Uninstall
    plist.go                  → PlistFor / RenderPlist
    logpath.go                → defaultLogPath
```

**Total:** ~1.855 LOC fonte, ~1.529 LOC teste (22 arquivos, ~62 testes).

---

## 4. REQUISITOS DO DAEMON

### 4.1 Gatilho de Emergência

O daemon roda via `launchd` como LaunchAgent de usuário, invocado a cada `check_interval_minutes` (default: 30 min). A cada execução:

1. **Checagem de espaço** via `statvfs` → `f_bavail` no volume configurado.
   - Usa `f_bavail` (espaço disponível a não-root), **não** `f_bfree` (espaço purgeable do APFS).
2. Se espaço livre < threshold, dispara a **Estratégia de Evicção**.
3. O polling mais frequente (60s) entre ciclos **não está implementado em V1** — o daemon é puramente periódico via launchd.

### 4.2 Estratégia de Evicção

**Ordem:**
1. Remove pastas inteiras em `enabled_orphan_apps` (se existirem).
2. Varre `safe_caches` e `logs`, ordena por mtime (desc) + size (desc), prioriza logs sobre caches.
3. Para quando espaço atinge `threshold + safety_margin_percent` **ou** `max_bytes_per_run`, o que vier primeiro.
4. Se espaço "não voltou" (arquivo ainda aberto), espera próximo ciclo.

### 4.3 Regras de Segurança

| Regra | Status V1 | Detalhes |
|---|---|---|
| Respeitar apps rodando | **Parcial** | `volatile_apps` checam bundle_id. Re-checagem antes de cada deleção **não implementada** |
| Detectar immutable/sflags | ✅ Feito | `UF_IMMUTABLE | SF_IMMUTABLE | UF_APPEND | SF_APPEND` |
| Permissões/TCC | ✅ Feito | Falha em item → log WARN + skip. Nenhum legível → log ERROR |
| Never logs secrets | ✅ Ok | Apenas mensagens formatadas |
| Detecção de órfãos | **Parcial** | Verifica existência no disco. Checar /Applications + processos **não implementado** |
| Hot-reload (SIGHUP) | **Não V1** | Recarrega config por ciclo naturalmente |

### 4.4 Alerta ao usuário

- Log persistente em `~/Library/Logs/thepurge/thepurge.log` (timestamp RFC3339).
- Notificação `osascript` quando eviction falha ou falta FDA: **não implementado em V1**.

---

## 5. CLI INTERATIVO

### Comandos V1 ✅

```
thepurge status                    Espaço livre (f_bavail), threshold
thepurge doctor                    Pré-flight: config, disk, targets, FDA
thepurge purge [--apply]           Evicção manual (default dry-run)
thepurge run [--apply]             Uma passagem (entrypoint do daemon)
thepurge config                    Resumo formatado
thepurge config --file             YAML colorido (ANSI)
thepurge config init               Cria config default
thepurge config validate           Valida parser + targets
thepurge config orphans.enable --path <PATH>
thepurge config orphans.disable --path <PATH>
thepurge service install [--apply] LaunchAgent
thepurge service uninstall         Remove LaunchAgent
thepurge service status            Verifica plist
thepurge logs [-n N] [-t|--tail]   Log
thepurge help
```

### Pendente (V2)

| Comando | Motivo |
|---|---|
| `config get`, `config set` | Stubs |
| `purge --force` | Aceito mas ignorado |
| TUI (Bubble Tea) | Requer dep externa |
| `status` mostrar último run | Requer state tracking |

---

## 6. LAUNCHD ✅

`~/Library/LaunchAgents/br4zz4.thepurge.plist`

Chaves: `Label`, `ProgramArguments` ([binpath, "run", "--apply"?]), `StartInterval`, `RunAtLoad` (true), `ProcessType` (Background), `StandardOutPath`/ `StandardErrorPath`.

Instalação: `launchctl bootstrap gui/$UID ~/Library/LaunchAgents/br4zz4.thepurge.plist`  
Remoção: `launchctl bootout gui/$UID ...` + remove plist

**Ordem obrigatória:** (1) `thepurge doctor` → (2) conceder FDA → (3) `thepurge service install`

---

## 7. ESTADO DOS TESTES

Todos os ~62 testes passam (go test ./...). Cobertura por pacote:

| Pacote | Testes | Cobre |
|---|---|---|
| config | 3 | Load spec fixture, default, marshal round-trip |
| eviction | 18 | Run (threshold, dry-run, apply), scan (ordem, min age, empty), delete (dry, apply, maxbytes, immutable), targets (home, subfolders, missing, orphans) |
| cli | 25 | Help, purge dry-run/apply, config orphans, logs -n, logs tail, summary, YAML color, integration |
| fsutil | 6 | ParseBytes, FormatBytes, DiskUsage, IsImmutable |
| logging | 3 | Write, append, create dir |
| service | 5 | Install, uninstall, plist content, plist validates, apply arg |

---

## 8. COMPILAÇÃO E USO

```bash
# Build
GOOS=darwin GOARCH=arm64 go build -o thepurge ./cmd/thepurge

# Inicializar config
./thepurge config init

# Verificar setup
./thepurge doctor

# Instalar serviço (dry-run)
./thepurge service install

# Evicção manual (preview)
./thepurge purge

# Evicção manual (executar)
./thepurge purge --apply

# Ver logs
./thepurge logs
```

> **Nota:** Sem Full Disk Access, o daemon roda mas não consegue ler `~/Library/*`. Execute `thepurge doctor` primeiro.
