# Bash idiom pack

> Authorities loaded by this pack are listed in §2; cite them in findings. Scope: **bash** scripts (`#!/usr/bin/env bash`), not POSIX `sh` — when a script's shebang is `sh`/`dash`, the bash-only idioms here (`[[ ]]`, arrays, `local`) become divergences, not rules (see §3). This repo's shell targets **macOS `/bin/bash` (3.2) + BSD userland**; that floor drives §5's portability overlay.

## 1. dimensions[]

| id | dimension | idiomatic rule | cue | authority |
|----|-----------|----------------|-----|-----------|
| D1 | Quoting & word-splitting | Quote every expansion (`"$var"`, `"${arr[@]}"`, `"$(cmd)"`) unless you *deliberately* want splitting/globbing — and when you do, say so. | a bare `$var`/`$(...)`/`$@` in a word context; `[ -z $x ]`; a path or glob built from an unquoted var | Google Shell §Quoting; ShellCheck SC2086/SC2046/SC2068 |
| D2 | Test & conditionals | `[[ ]]` for tests, `(( ))` for arithmetic; quote the LHS; prefer `if cmd; then` over testing `$?`. | `[ ]` in a bash script; `[ $x == $y ]`; `if [ $? -eq 0 ]` | Google Shell §Tests; Bash manual §Conditional Constructs; SC2181 |
| D3 | Command substitution | `$(...)` not backticks; never `local x=$(cmd)` (the `local` builtin's exit status masks the command's — splits declaration from assignment). | a `` `...` ``; `local x=$(...)` / `readonly x=$(...)` | Google Shell §Command Substitution; SC2006; SC2155 |
| D4 | Error handling & `set -e` | `set -euo pipefail` at the top of an *executed* script; know its holes — `set -e` is disabled inside a function/command used in a condition (`if f`, `f &&`, `f \|\|`), and `local x=$(cmd)` swallows failure. Guard with explicit `cmd \|\| return 1`. | a script with no strict-mode header; reliance on `set -e` inside an `if`-tested function; `$?` plumbing | Google Shell §Error Handling; BashFAQ/105 (set -e pitfalls); SC2164 |
| D5 | Loops & input | Iterate arrays/globs or `while IFS= read -r line`; never parse `ls`; `read` always takes `-r`. | `for f in $(ls ...)`; `for x in $dir/*` unquoted; `read` without `-r` | Google Shell §Loops; BashFAQ/001; SC2045/SC2162 |
| D6 | Arrays | Use real arrays and quote `"${arr[@]}"`; build them with `mapfile -t` (bash 4+) or a `while IFS= read -r` loop on the 3.2 target (§5 B1) — not `arr=($(cmd))` (word-splits + globs). Don't expand an array without an index. | `arr=($(...))`; `"$arr"` where `arr` is an array; a space-delimited string used as a list | Google Shell §Arrays; SC2128/SC2207 |
| D7 | Parameter expansion over externals | Prefer shell builtins (`${v%/}`, `${v##*/}`, `${v//a/b}`, `${v:-default}`) to spawning `sed`/`awk`/`cut`/`basename` for trivial string work. | `$(echo "$x" \| sed ...)`; `$(basename "$x")` for a simple suffix strip; a useless `cat`/`echo` in a subshell | Google Shell §Pipes; BashFAQ/073; SC2001/SC2116 |
| D8 | `printf` over `echo` for data | `printf '%s\n' "$x"` for anything that could contain `-`, `\`, or a leading flag; reserve `echo` for fixed literals. Never put data in `printf`'s format string. | `echo "$untrusted"`; `echo -e`/`-n` portability assumptions; `printf "$msg"` | Google Shell §STDOUT/STDERR; SC2059 |
| D9 | Functions & scope | Declare with `name() { … }`; `local` every function variable; `local` only works inside a function; small composable functions, `return` (not `exit`) for status. | a global mutated as a de-facto return; `local` at file scope; `function` keyword (ksh-ism) | Google Shell §Function Names/§Local Variables; SC2168 |
| D10 | Naming | `lower_snake_case` for locals/functions, `UPPER_SNAKE` for exported/env/constants (`readonly`/`declare -r`); never `UPPER` for an ordinary local (clobbers env). | an `UPPER` loop var; a `lower` constant; an unexported global that shadows a common env var | Google Shell §Naming Conventions |
| D11 | Safe file & process ops | Quote all paths; guard destructive vars (`rm -rf "${dir:?}"`); stage-then-atomic-`mv` within one directory; `mktemp` with a template; `trap … EXIT` for cleanup. | `rm -rf "$dir/"` with possibly-unset `dir`; a non-atomic write to a live file; `mktemp` in `$TMPDIR` then `mv` across filesystems | Google Shell §Pipelines; BashFAQ/062; SC2115 |
| D12 | Comment discipline *(required)* | Comments are an **uncommon exception** — names + structure carry intent; a comment earns its place only for a non-obvious *why* above the code. Top-of-file/function header docs are the sanctioned form; per-line narration is the smell. **No historical/changelog reasoning in source** ("we used to…", "removed because…"); **a deletion gets no tombstone** (no "load-bearing context for the removal" exception); the code states only *current* intent. A sourced library carries a header block, not a shebang. | a `# what` restating the next line; `# we used to …`/`# previously`; a comment naming a removed block or why; commented-out code; a comment a rename would delete | PLT-626 comment-discipline standard (`references/comment-discipline.md`); Google Shell §Comments |

## 2. authorities[]

- **Google Shell Style Guide** — https://google.github.io/styleguide/shellguide.html — the primary bash convention corpus (quoting, tests, naming, functions, pipelines, when-not-to-use-shell).
- **ShellCheck wiki** — https://www.shellcheck.net/wiki/ — per-`SC####` rationale + the idiomatic rewrite; the citable home of every §7 anchor.
- **Bash Reference Manual (GNU)** — https://www.gnu.org/software/bash/manual/bash.html — the normative semantics (conditional constructs, parameter expansion, arrays, shell options).
- **Greg's Wiki / BashFAQ & BashPitfalls** — https://mywiki.wooledge.org/BashFAQ , https://mywiki.wooledge.org/BashPitfalls — the canonical pitfall catalog (parsing `ls`, `set -e` holes, word-splitting arrays).
- **POSIX Shell Command Language** — https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html — the portability baseline for §3/§5 (what is bash-only vs portable).

## 3. divergences[] — where bash rejects general software wisdom

- **"Redundant quoting is noise" (terseness).** General style trims the obviously-safe. Bash says quote **every** expansion anyway — `"$var"` even when "it can't contain spaces" — because the failure is silent word-splitting/globbing, not a syntax error. → Reviewer must NOT flag defensive quoting as over-verbose; flag the *absence* of it.
- **"Decompose into many small functions / objects" (Clean Code, OOP).** A straight-line script with a few pipelines is often the *more* idiomatic shape than an over-functioned one; shell has no objects and function call overhead/scoping quirks (`local`, dynamic scope) make deep decomposition cost more than it pays. → Don't flag a cohesive linear script for "lacking abstraction"; flag genuine duplication only.
- **"Prefer portable POSIX `sh`."** When the shebang is `bash`, `[[ ]]`, arrays, `local`, and `${var/…}` are the idiom — *not* something to "port to `[ ]` for portability." → Don't flag bash features in a bash script. (The reverse holds in a `sh`/`dash` script — there they ARE divergences.)
- **"`set -e` makes error handling automatic."** Treating `set -e` as a complete safety net is wrong in shell: it is silently disabled for a command in a condition and masked by `local x=$(cmd)`. → Don't bless a script as safe merely because it has `set -e`; check the holes (D4).
- **"Comments explain the code."** Per the PLT-626 standard, comments are an exception, not a habit; names carry intent. → Don't recommend adding explanatory comments to self-evident shell; flag history/tombstone/what-comments.

## 4. anti_patterns[]

- **Parsing `ls`** — cue: `for f in $(ls)` / `ls \| while read`. Rewrite: glob (`for f in ./*`) or `find … -print0 \| while IFS= read -r -d ''`. (SC2045)
- **Unquoted expansion** — cue: `$var`/`$(cmd)`/`$@` in a word context. Rewrite: quote it; use `"$@"`, `"${arr[@]}"`. (SC2086/SC2046/SC2068)
- **`local x=$(cmd)`** — cue: declaration+assignment+substitution on one line under `set -e`. Rewrite: `local x; x=$(cmd)` so the command's exit status isn't masked by `local`. (SC2155)
- **Backticks** — cue: `` `cmd` ``. Rewrite: `$(cmd)` (nests, clearer). (SC2006)
- **`echo` for arbitrary data** — cue: `echo "$x"` where `$x` may start with `-` or hold `\`. Rewrite: `printf '%s\n' "$x"`. (SC2059 for the format-string variant)
- **Checking `$?`** — cue: `cmd; if [ $? -eq 0 ]`. Rewrite: `if cmd; then`. (SC2181)
- **`cd` without a guard** — cue: bare `cd "$d"` in a script. Rewrite: `cd "$d" || return 1` (or `exit`). (SC2164)
- **Building an array by word-splitting** — cue: `arr=($(cmd))`. Rewrite: `mapfile -t arr < <(cmd)` (bash 4+). On bash 3.2 (no `mapfile`), use a line loop for multi-line output — `arr=(); while IFS= read -r line; do arr+=("$line"); done < <(cmd)`; reserve `read -ra arr <<< "$line"` for splitting a **single** line into fields (it reads only one record — NOT a multi-line `mapfile` substitute). (SC2207 — see §5 B1)
- **Unguarded destructive path** — cue: `rm -rf "$dir/"` where `dir` may be empty/unset. Rewrite: `rm -rf "${dir:?dir is required}"`. (SC2115)
- **`eval` / building code from data** — cue: `eval "$user_input"`. Rewrite: arrays for argv, namerefs/`declare` for indirection — almost never `eval`. (judgment-only — see §7)

## 5. framework_overlays[]

### strict-mode (`set -euo pipefail`)
| id | rule | cue | consequence |
|----|------|-----|-------------|
| S1 | An *executed* script sets `set -euo pipefail` near the top; a *sourced* library does **not** (it would mutate the caller's shell). | a top-level script with no strict-mode; a sourced lib that re-sets shell options | unset-var typos and mid-pipeline failures pass silently; or a sourced lib corrupts the parent's options |
| S2 | Under `set -u`, reference maybe-unset vars as `"${x:-}"`; under `set -e`, a function in an `if`/`&&`/`\|\|` runs with `-e` *disabled* inside. | `"$maybe_unset"` with `set -u`; relying on `set -e` to abort inside an `if`-tested function | a guard you think aborts the script silently continues |
| S3 | `local x; x=$(cmd)` — never `local x=$(cmd)` — or the failure is swallowed even under `set -e`. | `local`/`readonly`/`export` `x=$(cmd)` | `set -e` does not fire on the failed command (D3/D4) |

### bsd-macos-userland (this repo's target)
| id | rule | cue | consequence |
|----|------|-----|-------------|
| B1 | Target macOS `/bin/bash` **3.2**: no `mapfile`/`readarray`, no associative arrays (`declare -A`), no `${var^^}`/`,,` case-conversion, no `&>>`-style novelties. Build multi-line arrays with a `while IFS= read -r` loop, not `mapfile` (§4). | a bash-4+ builtin/syntax in a script that must run on stock macOS | runs in dev CI on Linux, breaks on a macOS operator's shell |
| B2 | BSD vs GNU userland differs: `sed -i` needs a backup-suffix arg on BSD; `date`, `readlink -f`, `stat`, `grep -P` diverge. Prefer the portable form or `perl -pi -e`. | `sed -i 's/…/…/' file`; `readlink -f`; `grep -P` | the script works on the author's Linux box and corrupts/erros on macOS |

### sourced-library
| id | rule | cue | consequence |
|----|------|-----|-------------|
| L1 | A sourced lib has no shebang, declares only functions/constants, does not `set` shell options, and callers mark the source with `# shellcheck source=path` (or accept SC1091). | a sourced file with a shebang or top-level side effects; an unfollowed `source`/`.` with no directive | side effects fire at source time; SC1091 noise hides real findings |

## 6. severity_model

- **correctness:** unquoted expansion that changes behavior in a path/glob/argv context (D1 / SC2086/SC2046/SC2068); unguarded `rm -rf` (D11 / SC2115); `local x=$(cmd)` masking failure under `set -e` (D3/S3 / SC2155); a `set -e` guard silently disabled in a condition (D4/S2); parsing `ls` where filenames vary (D5/SC2045); a bash-4 feature on the 3.2 target (B1).
- **idiom-divergence-with-consequence:** backticks (D3/SC2006); `echo` for data / `printf "$var"` (D8/SC2059); `$?` plumbing (D2/SC2181); `arr=($(...))` splitting (D6/SC2207); a GNU-ism on the BSD target (B2); `cd` without a guard (D5/SC2164).
- **style:** naming (D10); `[[ ]]`-vs-`[ ]` where both are safe; function placement/ordering; comment nits that aren't history/tombstone (D12); parameter-expansion-vs-`sed` for a one-off where clarity is a wash (D7).

## 7. lint_anchors[]

The machine-checkable rules behind the dimensions. **Cite an anchor only from this table** — never assert an `SC` code from memory. Every code below was **verified by running `shellcheck` 0.11.0 (`-s bash`) on a bad snippet and observing the diagnostic**; re-verify against the repo's shellcheck version before locking one in. Where a dimension is **judgment-only**, there is no checkable rule — say so and cite the §2 prose authority; do **not** fabricate a code.

Catalog legend: **SC** = ShellCheck (https://www.shellcheck.net/wiki/SC####). ShellCheck is the one linter here; severity/enablement is largely default-on, but several codes below are *style/info* level and a repo may downgrade them.

| dimension / anti-pattern | anchor(s) | flags | caveat |
|---|---|---|---|
| D1 Quoting | `SC2086`; `SC2046`; `SC2068` | unquoted expansion (word-split/glob); unquoted `$(...)`; unquoted `$@` | the flagship check; `SC2086` is high-signal. Suppressed intentionally only with a `# shellcheck disable` + reason |
| D2 Tests / `$?` | `SC2181` | testing `$?` instead of the command directly | `[[ ]]`-vs-`[ ]` *preference* has **no code** (both parse) — judgment-only, cite Google Shell §Tests |
| D3 Command substitution | `SC2006`; `SC2155`; `SC2116` | legacy backticks; `local/readonly/export x=$(cmd)` masking exit status; useless `echo` in `$(...)` | `SC2155` is the `set -e`-masking anchor — strong, safe to cite |
| D4 Error handling | `SC2164` | `cd` without `\|\| exit`/`return` | the deeper `set -e` holes (disabled-in-condition, pipefail semantics) are **judgment-only** — no code models them; cite BashFAQ/105 |
| D5 Loops & input | `SC2045`; `SC2035`; `SC2162` | iterating `ls`; unanchored glob (`*.x` → `./*.x`); `read` without `-r` | `SC2035` is style-level |
| D6 Arrays | `SC2128`; `SC2207` | expanding an array without an index; `arr=($(...))` split-and-glob | `SC2207`'s suggested `mapfile` is **bash-4+**; on the 3.2 target use a `while IFS= read -r` line loop for multi-line output — `read -ra` reads only a **single** line (field-split), not a multi-line `mapfile` substitute (§5 B1) |
| D7 Param-expansion vs externals | `SC2001`; `SC2116` | `sed` for a substitution a `${//}` would do; useless `echo`/`cat` | `SC2001` is often a *judgment* call (sed is fine for real regex work) — cite only for the trivial case; `SC2002` (useless `cat`) exists but is frequently disabled — treat as style |
| D8 `printf` vs `echo` | `SC2059` | variables in `printf`'s format string | "echo-for-data" generally is partly judgment; `SC2059` catches the dangerous format-string case precisely |
| D9 Functions & scope | `SC2168` | `local` used outside a function | `function` keyword and decomposition depth are **judgment-only** |
| D11 Safe file ops | `SC2115` | `rm -rf "$x/"` where `$x` may be empty (`${x:?}` guard) | atomicity / same-dir-`mv` / `mktemp`-template discipline is **judgment-only** — no code; cite Google Shell + BashFAQ/062 |
| D12 Comment discipline | — | — | **judgment-only** — ShellCheck has no commented-out-code or narrative-comment check (no gocritic `commentedOutCode` equivalent); cite `comment-discipline.md` (PLT-626) + Google Shell §Comments |
| (hygiene) unused / undefined | `SC2034`; `SC2154` | assigned-but-unused var; referenced-but-never-assigned var | `SC2154` is a strong typo-catcher |
| (hygiene) grep pipelines | `SC2126` | `grep … \| wc -l` → `grep -c` | style-level |
| (hygiene) sourced libs | `SC1091`; `SC1090` | a **literal**-path `source`/`.` whose target isn't followed (`SC1091`); a **variable**-path `source "$x"` (`SC1090`) | resolve with `# shellcheck source=path`, not by disabling — see L1 |
| AP `eval` / dynamic code | — | — | **judgment-only** — ShellCheck warns on some `eval` shapes but won't catch injection generally; cite Google Shell §Eval (avoid) |

**Genuinely judgment-only — never fabricate a code:** the `[[ ]]`-vs-`[ ]` preference; the `set -e` holes (disabled-in-condition, pipefail nuance); function decomposition depth and the `function` keyword; atomic-write / same-dir-`mv` / `mktemp`-template discipline; comment discipline (D12) in full; `eval`/injection; the bash-3.2 / BSD-userland portability rules (B1/B2) — ShellCheck's `# shellcheck shell=` directive catches *some* non-bash constructs but does **not** enforce a bash-version floor or BSD-vs-GNU userland. For all of these, cite the §2 prose authority (Google Shell / BashFAQ / Bash manual / the repo's "bash 3.2 compatible" convention) and state that no machine-checkable rule exists.

## Language → specialist agent

| language | specialist agent |
|----------|------------------|
| bash | `platform-engineer` (shell + BSD/macOS userland; the sync-script and runtime-shell surfaces) |
