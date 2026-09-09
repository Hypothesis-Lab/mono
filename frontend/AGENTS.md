# Conventions

## Project rules
src/config/ - accessing env vars only from here

## Organizing pages

```
src/pages/<PageName>/
    page.tsx
    styles.ts
    components/<Component1>/ // optionally if it page has components that it uses, these components are not used by other pages
        component.tsx
        styles.ts
src/components/<Component1>/ // for components reused between pages
    page.tsx
    styles.ts

```

## Installing packages

Install the latest version by default. Pin only if explicitly requested.


## Responsivness guide
- 640px: large phones / small tablets
- 768px: tablet
- 1024px: laptop / desktop
- 1280px: large desktop

## Verification
After making changes, run linters, tests, build

## Backend API reference
../backend/internal/api/spec.yaml

backend patch routes generally can't distinguish between null and absent fields, so when patching use this rule:
- if field_changed send that field, even if it's empty string
- else don't send the field
