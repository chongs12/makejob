---
name: spec-writer
description: Interactive specification drafting for product and feature work. Use when the user types `/spec` or asks to analyze requirements and produce a Markdown specification document. This skill drives requirement discovery, clarifies scope and constraints, inspects the existing codebase when relevant, and outputs a spec with technical stack, API definitions, risks, open questions, and acceptance criteria.
---

# Spec Writer

Turn rough feature ideas into implementation-ready Markdown specs.

## Workflow

1. Establish context first.
- Inspect the repository when it exists.
- Reuse the current stack, folder conventions, auth model, and API style before proposing new technology.
- If the user only entered `/spec`, treat that as a request to start guided discovery.

2. Run requirement discovery in compact rounds.
- Ask only the missing high-value questions.
- Prioritize: problem being solved, target users, core flows, scope boundaries, constraints, integrations, auth and roles, main entities, and delivery expectations.
- Keep questions grouped and short so the conversation keeps moving.
- When details are missing but drafting must continue, state explicit assumptions instead of hiding uncertainty.

3. Draft the spec using `references/spec-template.md`.
- Fill only the sections that matter for the request.
- Keep the document practical and implementation-oriented.
- Prefer concrete decisions over vague placeholders.

4. Define the technical stack carefully.
- If a stack already exists, document the current stack and list only the additions or changes needed.
- If the request is greenfield, recommend a concise stack that matches the requested scope and constraints.
- Mention frontend, backend, database, auth, deployment, and key third-party services only when they are relevant.

5. Define APIs precisely when the feature needs them.
- For each endpoint, include method, path, purpose, auth requirements, request fields, response fields, and important error cases.
- Keep naming aligned with existing routes and payload shapes when a codebase already exists.
- Call out whether an API is new, changed, or reused.

6. Write testable acceptance criteria.
- Make them observable and verifiable.
- Cover user-visible behavior, permission behavior, validation rules, and key failure cases.
- Prefer checklist-style statements such as "Given/When/Then" or equivalent measurable outcomes.

7. Close with execution-ready output.
- Save the result as a Markdown document if the user asked for a file.
- If no path was provided, suggest a practical location such as `docs/<feature-name>-spec.md` or `spec.md`.
- End with open questions, assumptions, and the recommended first implementation slice.

## Discovery Checklist

Use this checklist to decide what still needs clarification:

- Business goal: What problem is being solved and why now?
- Users and roles: Who uses it and what permissions differ?
- Scope: What is explicitly in scope and out of scope?
- Core flows: What are the main user journeys?
- Data: What entities, fields, and relationships matter?
- Integrations: What external systems or internal modules are involved?
- Constraints: What tech, deadline, compliance, performance, or compatibility limits apply?
- Delivery: Is the goal a prototype, production-ready feature, refactor, or replacement for mock data?

## Output Rules

- Produce Markdown, not prose notes.
- Use clear section headings.
- Mark assumptions explicitly.
- Keep API definitions and acceptance criteria concrete enough for implementation and QA.
- Avoid speculative architecture that is not justified by the request or codebase.

## Reference

- Use `references/spec-template.md` as the default structure for the final document.
