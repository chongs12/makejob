# Spec Template

Use this template when drafting the final spec. Remove sections that do not apply and replace prompts with concrete content.

```md
# <Feature or Project Name> Spec

## Summary
- What is being built
- Why it matters
- Current context or problem statement

## Goals
- Goal 1
- Goal 2

## Non-Goals
- Explicitly out-of-scope item 1
- Explicitly out-of-scope item 2

## Users and Roles
- Primary users
- Admin or operator roles
- Permission differences that matter

## Core User Flows
1. Flow one
2. Flow two
3. Edge or failure flow

## Functional Requirements
- Requirement 1
- Requirement 2
- Validation rules
- State transitions or business rules

## Non-Functional Requirements
- Performance
- Security
- Observability
- Compatibility

## Technical Stack
- Frontend:
- Backend:
- Database:
- Auth:
- Infrastructure / deployment:
- Third-party services:

## Data Model Notes
- Entity:
  - key fields
  - relationships
- Entity:
  - key fields
  - relationships

## API Definitions
### <Endpoint Name>
- Method: `GET|POST|PUT|PATCH|DELETE`
- Path: `/api/...`
- Purpose:
- Auth:
- Request:
  - field:
    - type:
    - required:
    - notes:
- Response:
  - field:
    - type:
    - notes:
- Errors:
  - `400`:
  - `401/403`:
  - `404`:

## Frontend Notes
- New pages, components, routes, or state changes
- Interaction with existing layouts or design system

## Backend Notes
- New modules, services, repositories, jobs, or migrations
- Impact on existing APIs or auth rules

## Risks and Open Questions
- Risk:
- Open question:

## Acceptance Criteria
- [ ] Criterion 1
- [ ] Criterion 2
- [ ] Criterion 3

## Suggested Implementation Order
1. Slice 1
2. Slice 2
3. Slice 3
```
