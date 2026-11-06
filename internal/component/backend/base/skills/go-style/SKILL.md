---
name: go-style
description: "Modern Go idioms for every Go file: which standard-library and language features to use instead of the older pattern, gated by the Go version in go.mod."
---

# Skill: Go style

Models write Go the way most of their training data is written: `interface{}`, `sort.Slice`,
`for i := 0; i < n; i++`. This skill lists the modern replacements, and which Go version each
needs.

The rules are JetBrains' [Modern Go Guidelines](https://github.com/JetBrains/go-modern-guidelines)
(Apache-2.0), copied from **v0.1.1, commit `155dc7c`**: licensed under the
[Apache License 2.0](https://www.apache.org/licenses/LICENSE-2.0), and adapted here, reworded and
without the parts that do not apply.

## When this skill applies

Writing, changing or reviewing any `.go` file. How code is layered is the api-design skill's, how
errors travel the error-handling skill's, how tests are built the testing skill's: this one says
which idiom to write it in.

## How to use the tables

1. **Read the `go` line in `go.mod`.**
2. **Use every rule at or below that version** in the code you write or change, even when the code
   around it uses the older pattern.
3. **Never use a rule above it.** It does not compile, or the API does not exist yet.
4. **Skip a rule only** when it would change behaviour or plainly does not fit, and say why in the
   MR.

Rewriting existing code only to modernise it is a change of its own, never a side effect of
another one.

## Go 1.25 and older

| Go | Rule |
|---|---|
| 1.0 | `time.Since(start)`, not `time.Now().Sub(start)` |
| 1.8 | `time.Until(deadline)`, not `deadline.Sub(time.Now())` |
| 1.13 | `errors.Is(err, target)`, not `err == target` — wrapped errors must still match |
| 1.18 | `any`, not `interface{}` |
| 1.18 | `strings.Cut` / `bytes.Cut`, not `Index` plus manual slicing |
| 1.18 | `strings.Clone` to copy a string without keeping its backing memory alive |
| 1.19 | `fmt.Appendf` when appending formatted text to a `[]byte`, not `append(b, fmt.Sprintf(…)...)` |
| 1.19 | Typed atomics — `atomic.Bool`, `atomic.Int64`, `atomic.Pointer[T]` — not the `atomic.AddInt64` functions |
| 1.20 | `bytes.Clone` to copy a byte slice |
| 1.20 | `strings.CutPrefix` / `CutSuffix` when you need the trimmed string *and* whether it matched |
| 1.20 | `errors.Join` to combine several errors so `errors.Is` still sees each of them |
| 1.20 | `context.WithCancelCause` and `context.Cause` when a cancellation should carry its reason |
| 1.21 | Built-in `min` and `max`, not hand-written comparisons |
| 1.21 | `clear(m)` to empty a map, `clear(s)` to zero a slice |
| 1.21 | `slices.Contains`, not a search loop |
| 1.21 | `slices.Index` / `slices.IndexFunc` to find an element's position |
| 1.21 | `slices.Sort` for ordered values; `slices.SortFunc` with `cmp.Compare`, not `sort.Slice` — and `slices.SortStableFunc` where `sort.SliceStable` was, because `SortFunc` is not stable |
| 1.21 | `slices.Max` / `slices.Min`, not a loop |
| 1.21 | `slices.Reverse`, not a swap loop |
| 1.21 | `slices.Compact` to drop consecutive duplicates |
| 1.21 | `slices.Clip` to drop unused capacity; `slices.Clone` to copy a slice |
| 1.21 | `maps.Clone` / `maps.Copy`, not a copy loop; `maps.DeleteFunc` to delete by predicate |
| 1.21 | `sync.OnceFunc` / `sync.OnceValue`, not `sync.Once` plus a wrapper closure or a cached variable |
| 1.21 | `context.AfterFunc` to run cleanup when a context is cancelled |
| 1.21 | `context.WithTimeoutCause` / `WithDeadlineCause` when a caller needs to know why it timed out |
| 1.22 | `for i := range n`, not `for i := 0; i < n; i++` |
| 1.22 | No `v := v` copies before a closure or `&v` — every iteration has its own variable |
| 1.22 | `cmp.Or(a, b, c)` for the first non-zero value, not an `if` chain |
| 1.22 | `reflect.TypeFor[T]()`, not `reflect.TypeOf((*T)(nil)).Elem()` |
| 1.22 | Method-aware `http.ServeMux` patterns and `r.PathValue` — only where plain `net/http` is used; routes here are Gin |
| 1.23 | `maps.Keys` / `maps.Values` as iterators, not a loop that collects them |
| 1.23 | `slices.Collect` to build a slice from an iterator; `slices.Sorted` to collect and sort in one step |
| 1.23 | `time.Tick` where it fits — an unreferenced ticker is garbage-collected without `Stop` |
| 1.24 | `t.Context()` for a context that lives as long as the test — it is already cancelled when `t.Cleanup` runs, so never pass it to cleanup |
| 1.24 | `omitzero` on bool, number, struct and `time.Time` JSON fields; `omitempty` stays for strings, slices and maps |
| 1.24 | `for b.Loop()` in benchmarks — a benchmark that sizes its fixtures from `b.N` needs reworking, not a mechanical swap |
| 1.24 | `strings.SplitSeq` / `FieldsSeq` (and the `bytes` versions) when you only range over the parts |
| 1.25 | `wg.Go(func() { … })`, not `wg.Add(1)` plus `go func() { defer wg.Done() … }()` |

## Go 1.26 and newer

| Go | Rule |
|---|---|
| 1.26 | `new(value)` for a pointer to a value, not a `ptr()` helper or a temporary variable |
| 1.26 | `errors.AsType[T](err)`, not `errors.As` with a pre-declared target |
| 1.27 | Generic methods on the type, not package-level generic helpers, when the operation belongs to the type |
| 1.27 | `encoding/json/v2` for new JSON code; existing `encoding/json` code stays unless the Issue is the migration |
| 1.27 | Promoted field names directly in a struct literal, not the embedded struct built explicitly |
| 1.27 | `strings.CutLast` / `bytes.CutLast`, not `LastIndex` plus manual slicing |
| 1.27 | The standard library `uuid` package, not a third-party one |
| 1.27 | `URL.Clone` and `Values.Clone`, not a manual copy |

## When Go is upgraded

In the change that raises the `go` line in `go.mod`:

1. Raise the `golang:` image tag in `docker-compose.yaml` with it. The image does not download a
   newer Go, so a `go` line above its version stops the stack from starting.
2. Move the rows the new version unlocks into the first table.
3. Compare upstream's latest release against the pin above, and add its new rules.

## Definition of done

- [ ] New and changed Go code uses every rule at or below the `go.mod` version that fits it
- [ ] No rule above the `go.mod` version is used
- [ ] A rule that fits but was skipped is explained in the MR
