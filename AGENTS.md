## Type Safety

- Never use explicit type casting, including `as`, angle-bracket casts, and non-null assertions used to bypass type checking.
- Prefer precise schemas, constructors, type guards, narrowing, and typed decoding instead of casts.
- Prefer Effect Schema definitions over `type` or `interface` definitions.
- Do not inspect Effect data structure tags like `_tag` directly. Use Effect helpers such as `Either.isRight`, `Either.isLeft`, `Option.isSome`, `Option.isNone`, or `Match` APIs.
- Never use `new` for Effect Schema classes except `Schema.TaggedError` classes. Prefer the `.make()` constructor.
- Extract schema types with `Schema.Schema.Type<typeof MySchema>` instead of `typeof MySchema.Type`.

## Testing

- Run `pnpm typecheck` after each implementation turn to maintain an immediate type-safety feedback loop.
- Keep tests close to the implementation at the same level, using `*.spec.ts` files.
- Use `@effect/vitest` for tests.
- For tests that run Effect programs, prefer `it.effect` so assertions execute inside the Effect runtime and TestContext is available.
- For expected typed Effect failures, prefer `Effect.flip` inside `it.effect` to assert the error directly.
- Use `it.live` only when the test intentionally needs the live Effect environment, such as real clock or logging behavior.
- Use `it.scoped` / `it.scopedLive` only for tests that require `Scope` or scoped resource lifecycles.
- Do not use `it.flakyTest` to hide non-determinism; first prefer making tests deterministic.
- Keep pure schema and domain tests as plain `it` unless they execute an Effect.
- Avoid internal helper functions in tests; prefer direct, explicit setup in each spec unless duplication becomes clearly harmful.

## Design

- Always strive for YAGNI. Prefer the smallest design and implementation that satisfies the current requirement.
- Respect architectural dependency direction at all times. This is about dependencies between project layers, not normal framework or library imports used in the correct layer. Domain must not import application or infrastructure code. Application may depend on domain and application-owned ports, while still using appropriate libraries such as Effect. Infrastructure may depend on application ports and domain types to implement adapters, and may use adapter-specific libraries such as `@effect/platform` or `@effect/sql` behind those ports.
- Avoid unrelated cross-layer imports; if a dependency feels strange, stop and redesign the boundary.
- Prefer straightforward Effect code over clever abstractions. Readable and understandable code is better than complex or "smart" code, even when the clever version is shorter. Use direct `Effect.gen`, small semantic helpers, and explicit data flow unless an abstraction clearly reduces repeated complexity.
- Prefer `function` declarations for named helpers instead of assigning arrow functions to constants.
- When working on `if` conditions, check Effect `Predicate` helpers first and prefer them when they make the condition clearer. For multi-clause predicates, consider combinators like `Predicate.and`, `Predicate.or`, and `Predicate.not` to make the flow more readable.
- Prefer Effect `Data.array` over JavaScript `Array` when constructing immutable arrays as data, especially for value equality.
- Use PascalCased directories for all new code under `src`, including layer directories like `src/Domain`, `src/Application`, and `src/Infrastructure`.
- Use PascalCased feature directories under application and infrastructure boundaries when the directory represents a named port, service, or adapter, such as `src/Application/Ports/ModelGarden`.
- Avoid hyphenated feature directories for new application and infrastructure code.
- Keep domain models focused on intrinsic state and behavior. For example, `Model` and `ModelCapabilities` belong in the domain; a catalog that resolves models from config or external metadata does not.
- Define domain events with an explicit `type` field using `Schema.Class`, not `_tag` or `Schema.TaggedClass`. Reserve `Schema.TaggedError` for typed errors.
- Structure application code around command DTOs, services, and driven ports:
  - Commands are validated application-layer input DTOs consumed by services.
  - Services contain use-case workflow and coordinate domain models with driven ports.
  - Ports are application-owned contracts for external capabilities, such as `ModelGarden` or `AgentRuntime`.
- Define every application port in `src/Application/Ports` as an Effect service tag (`Context.Tag`), not as a plain TypeScript interface.
- Keep HTTP DTOs and transport-specific decoding in HTTP adapters unless a DTO is the application command itself.
- Keep database migrations close to the infrastructure adapter that owns the schema, use the shared `_migrations` table, and name migrations with timestamp-style globally unique IDs such as `20260428120000_event_journal_create_domain_events`.
- Prefer Pattern Matching via Effect over `switch` statements.

## Vendored Repositories

This project vendors external repositories under `@repos/`.

- Use vendored repositories as read-only reference material when working with related libraries.
- Prefer examples and patterns from the vendored source code over generated guesses or web search results.
- When writing Effect code, inspect `@repos/effect/` for examples of idiomatic usage, tests, module structure, and API design. Treat it as the source of truth for Effect patterns.
- Do not edit files under `@repos/` unless explicitly asked.
- Do not import from `@repos/`; application code should continue importing from normal package dependencies.

<!--VITE PLUS START-->

# Using Vite+, the Unified Toolchain for the Web

This project is using Vite+, a unified toolchain built on top of Vite, Rolldown, Vitest, tsdown, Oxlint, Oxfmt, and Vite Task. Vite+ wraps runtime management, package management, and frontend tooling in a single global CLI called `vp`. Vite+ is distinct from Vite, and it invokes Vite through `vp dev` and `vp build`. Run `vp help` to print a list of commands and `vp <command> --help` for information about a specific command.

Docs are local at `node_modules/vite-plus/docs` or online at https://viteplus.dev/guide/.

## Review Checklist

- [ ] Run `vp install` after pulling remote changes and before getting started.
- [ ] Run `vp check` and `vp test` to format, lint, type check and test changes.
- [ ] Check if there are `vite.config.ts` tasks or `package.json` scripts necessary for validation, run via `vp run <script>`.

<!--VITE PLUS END-->
