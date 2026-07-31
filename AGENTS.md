# Project Engineering Rules

These rules apply to all code changes unless the task explicitly overrides them.

The file and directory limits are architectural constraints, not cleanup rules. Plan the implementation so the limits are satisfied from the beginning. Do not write a large file first and split it only after reaching the limit.

## 1. Before Coding

Before modifying or creating code:

1. Read this `AGENTS.md` and all relevant project documentation.
2. Inspect the existing:

   * architecture;
   * directory structure;
   * naming conventions;
   * dependencies;
   * public APIs;
   * tests;
   * build, lint, formatting, and type-checking commands.
3. Check available skills and use every skill relevant to the task.
4. Identify the affected feature, domain, layer, and integration boundaries.
5. Estimate the implementation size before writing code.
6. Produce an internal file plan that lists:

   * files to create;
   * files to modify;
   * the responsibility of each file;
   * expected dependencies between files;
   * approximate size of each new or substantially changed file.
7. Create the required directory structure before implementing substantial logic.
8. Prefer the simplest solution that remains cohesive, testable, and easy to extend.

Do not begin implementation until module boundaries and data flow are reasonably clear.

For trivial changes, the file plan may be brief. For multi-file features, integrations, UI flows, or changes expected to exceed roughly 150 lines, the file plan is mandatory.

## 2. Proactive Structure Planning

Treat file and directory limits as design inputs.

Before writing a new feature, determine whether it requires separate modules for concerns such as:

* public API or entry point;
* orchestration or use cases;
* domain rules;
* data models and schemas;
* persistence;
* external service adapters;
* UI or presentation;
* validation;
* error definitions;
* configuration;
* tests and fixtures.

Create these modules early when their responsibilities are already known. Do not keep unrelated logic in one temporary file with the intention of splitting it later.

When a feature is expected to require several modules, create a dedicated feature or domain directory at the start.

Example:

```text
src/
  billing/
    index.ts
    service.ts
    models.ts
    validation.ts
    adapters/
      payment-gateway.ts
```

The exact structure must follow the project’s language and conventions. Do not copy this example mechanically.

## 3. Architecture

* Separate presentation, application logic, domain logic, and infrastructure.
* Keep dependencies explicit and directed toward the domain.
* Each module, class, and function must have one clear responsibility.
* Do not mix UI, business logic, persistence, networking, and configuration in one file.
* Avoid global mutable state, circular dependencies, hidden side effects, and service locators.
* Use interfaces only at real boundaries or where multiple implementations are expected.
* Do not introduce abstractions, layers, factories, repositories, managers, or patterns without a concrete need.
* Organize code by feature or domain when it improves cohesion.
* Prefer local feature modules over expanding central registries, switches, controllers, or utility files.
* Keep framework-specific code at the system boundary.
* Keep domain behavior independent from transport, storage, UI, and third-party SDKs.
* Dependencies between sibling modules must remain intentional and acyclic.
* Public entry-point files should primarily export or compose functionality, not contain substantial business logic.

## 4. File Size Limits

### Hard limit

A source file must never exceed 300 lines.

The 300-line limit is a hard ceiling, not a target.

### Working limit

Plan ordinary source files to remain below 220–250 lines so later validation, error handling, and maintenance do not immediately force restructuring.

When a file approaches 220 lines:

1. stop adding unrelated behavior;
2. review its responsibilities;
3. move cohesive secondary responsibilities into dedicated modules before continuing.

Do not wait until line 299.

### Size estimation

Before creating a substantial file, estimate whether it will contain multiple independent concerns.

Split it in advance when it is likely to contain any combination of:

* several domain operations;
* transport and business logic;
* parsing and execution;
* data models and orchestration;
* provider-specific and provider-independent logic;
* multiple UI components;
* state management and rendering;
* implementation and extensive helper logic.

A file may remain larger only when its contents form one genuinely cohesive unit. It must still remain below 300 lines.

### Splitting rules

Split files by responsibility, behavior, lifecycle, or architectural boundary.

Good splits include:

* `parser.ts` and `executor.ts`;
* `service.ts` and `repository.ts`;
* `models.ts` and `validation.ts`;
* `controller.ts` and `use-case.ts`;
* `component.tsx` and `use-component-state.ts`;
* `client.ts` and `response-mapper.ts`.

Bad splits include:

* `helpers1.ts` and `helpers2.ts`;
* `service-part-a.ts` and `service-part-b.ts`;
* arbitrary line-range extraction;
* moving unrelated code into `utils.ts`;
* creating forwarding files that add no meaningful boundary.

Do not create “god files”, utility dumps, generic manager modules, or catch-all modules.

Imports, comments, and blank lines still count toward the limit unless the project’s verification script explicitly defines another counting method.

## 5. Directory Size Limits

A directory must contain no more than 5 source files.

Generated files, migrations, fixtures, snapshots, assets, vendored code, and external dependencies are exempt.

Before adding a fifth source file to a directory, evaluate whether the next expected change will require another file. If so, create meaningful subdirectories before continuing.

Split directories by real concepts such as:

* feature;
* domain;
* layer;
* adapter type;
* transport;
* provider;
* workflow;
* component group.

Example:

```text
notifications/
  index.ts
  service.ts
  models.ts
  channels/
    email.ts
    push.ts
  persistence/
    repository.ts
```

Do not create arbitrary folders such as `misc`, `common2`, `extra`, `parts`, or `more` merely to satisfy the file-count limit.

Do not hide an incoherent architecture behind deeply nested directories. Directory depth must reflect actual conceptual boundaries.

## 6. Implementation Workflow

For non-trivial changes, implement in this order:

1. establish directories and module boundaries;
2. define domain models, contracts, and public interfaces;
3. implement focused domain or application behavior;
4. add infrastructure and external adapters;
5. connect entry points, UI, routes, commands, or handlers;
6. add or update tests;
7. update documentation and version information when required;
8. run verification;
9. review file and directory limits again.

After each substantial module is completed:

* check its current line count;
* check the source-file count in its directory;
* verify that its responsibility still matches the original file plan;
* adjust the structure immediately if the estimate was wrong.

Do not postpone structural corrections until the end of the task.

If implementation reveals a new responsibility, create a dedicated module at that point instead of appending it to the nearest existing file.

## 7. Code Quality

* Write small, focused functions with explicit inputs and outputs.
* Use clear names instead of comments that compensate for confusing code.
* Keep public APIs minimal and stable.
* Validate external input at system boundaries.
* Handle errors explicitly and preserve useful context.
* Distinguish domain errors, validation errors, and infrastructure failures where useful.
* Avoid duplicated code, but do not generalize before duplication is proven.
* Prefer pure functions for domain transformations when practical.
* Keep side effects visible and localized.
* Remove dead code, obsolete comments, debug output, temporary compatibility code, placeholders, and unused dependencies.
* Do not leave unfinished TODOs unless explicitly requested.
* Do not modify unrelated code.
* Do not perform broad refactoring merely because nearby code could be cleaner.
* Avoid exporting implementation details solely to simplify tests.
* Keep tests readable and focused on observable behavior.

## 8. Extensibility

* New behavior should normally be added through a focused module, not by expanding a central switch, registry, controller, or god object.
* Keep domain rules independent from frameworks and external services.
* Isolate third-party APIs behind small adapters.
* Prefer composition over inheritance.
* Make configuration explicit, typed where possible, and environment-aware.
* Preserve backward compatibility unless a breaking change is explicitly approved.
* Design extension points only where variation is known or strongly expected.
* Do not add speculative interfaces, plugin systems, factories, or generic pipelines for hypothetical future requirements.
* When supporting multiple providers or strategies, keep shared policy separate from provider-specific implementation.
* Avoid central modules that must be edited for every new feature when registration or composition can remain local and explicit.

## 9. Tests and Verification

Cover critical business rules, boundary conditions, error paths, and regressions with tests.

* Test behavior, not internal implementation details.
* Place tests according to the project’s established convention.
* Keep test helpers focused; test directories are not dumping grounds.
* Add regression tests for fixed bugs when practical.
* Do not weaken, delete, or bypass tests merely to make a change pass.
* Do not replace meaningful assertions with snapshots unless snapshots are appropriate for the behavior.

Run all applicable project checks after changes:

* formatter;
* linter;
* type checker;
* unit tests;
* integration tests;
* build;
* architecture or dependency checks;
* file-size and directory-size checks.

Do not claim a check passed unless it was actually executed.

If a check cannot run, report:

1. the exact command;
2. the reason it could not run;
3. whether the failure is caused by the change or by the environment;
4. what remains unverified.

## 10. Automated Limit Verification

Use an existing project script for file and directory limits when available.

If the repository has no automated check and the task adds or substantially changes multiple source files, verify the limits with an appropriate command or small temporary inspection script.

Do not commit a new verification tool unless it is useful for ongoing project enforcement and fits the repository’s tooling.

The final verification must confirm:

* no non-exempt source file exceeds 300 lines;
* no non-exempt directory contains more than 5 source files.

A file already exceeding the limit before the task must not be made larger without explicit justification. When the task substantially modifies such a file, split it if doing so is within scope and does not create unrelated risk.

## 11. Documentation and Versioning

Maintain, when applicable:

* `README.md` — setup, usage, architecture, commands, and environment variables;
* `CHANGELOG.md` — meaningful user-facing and architectural changes;
* the project version in the standard ecosystem file, such as:

  * `package.json`;
  * `pyproject.toml`;
  * `Cargo.toml`;
  * another ecosystem-standard manifest.

Use a standalone `VERSION` file only when the ecosystem has no standard version field.

Update documentation when the change affects:

* setup;
* configuration;
* environment variables;
* commands;
* public APIs;
* user-visible behavior;
* architecture that maintainers need to understand.

Update version information only when required by the project’s release policy or explicitly requested.

Do not create documentation files containing empty boilerplate.

## 12. Scope Control

* Make the smallest coherent change that fully solves the task.
* Do not modify unrelated modules.
* Do not rename or move files without a concrete architectural reason.
* Do not introduce new dependencies when the standard library or existing dependencies are sufficient.
* Do not combine feature work with opportunistic cleanup.
* If an adjacent issue blocks the requested work, fix only the blocking portion and report it.
* Preserve existing conventions unless they conflict with these rules or cause a concrete problem.

## 13. Final Review

Before finishing, inspect the resulting structure rather than relying only on tests.

Verify that:

* no source file exceeds 300 lines;
* ordinary source files were designed with reasonable space below the hard limit;
* no directory contains more than 5 source files;
* new directories represent meaningful architectural concepts;
* files were split by responsibility rather than line count;
* responsibilities are not mixed;
* dependencies remain explicit and acyclic;
* public APIs remain minimal;
* the solution can be extended without rewriting unrelated modules;
* no central god object, utility dump, or oversized entry point was introduced;
* no unnecessary abstraction or dependency was added;
* no dead code, temporary debug output, placeholders, or unfinished TODOs remain;
* documentation reflects the current project;
* all applicable checks were executed.

If any item is not satisfied, correct it before producing the final response unless correction is impossible or outside the approved scope.

## 14. Final Response

In the final response, report:

1. **What changed**

   * files created;
   * files modified;
   * files moved or removed;
   * user-visible behavior affected.

2. **Architectural decisions**

   * module boundaries;
   * dependency direction;
   * why new directories or files were introduced;
   * how file and directory limits were handled proactively.

3. **Skills used**

   * list only skills actually used.

4. **Checks executed**

   * exact commands;
   * pass or fail result;
   * relevant warnings;
   * checks that could not run and why.

5. **Limit verification**

   * maximum source-file line count after the change;
   * directories at the 5-file limit;
   * confirmation that no limit was exceeded.

6. **Remaining risks or limitations**

   * unverified behavior;
   * compatibility concerns;
   * follow-up work that is genuinely outside scope.

Keep the report factual. Do not claim success for work or checks that were not completed.
