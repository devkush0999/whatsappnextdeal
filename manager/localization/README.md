# English manager text

This checkout contains only the prebuilt manager (`manager/dist`), not its React source or build configuration. The English copy is recorded in `en.json`; it is already applied to the bundled JavaScript and the passkey helper UI. Branding and attribution are retained.

The translation changes string literals and template text only. API routes, payload field names, event identifiers, option values, and application control flow are preserved. `index.html` includes a content-derived version query so browsers fetch the updated script.

When frontend source is restored, move these translations into its localization layer before rebuilding. Replacing the prebuilt bundle from upstream will otherwise replace these English strings. Server-provided errors and Swagger content are separate from manager UI copy.
