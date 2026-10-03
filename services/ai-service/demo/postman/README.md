# Postman helpers

## Two scripts, one per request

| Script | Paste into | Sets |
|---|---|---|
| `save-criteria.js` | `DeriveCriteriaFromDescription` | `criteria` |
| `save-profile.js` | `ExtractProfileFromText` | `profile` |

Each is standalone. They duplicate the routine that finds the payload inside `pm.response`,
which is the price of keeping each script about one request — if Postman moves the response
accessor again, both need the same fix.

**Neither file may contain a variable reference in doubled braces**, not even in a comment.
Postman substitutes those before evaluating the script, so a saved JSON value gets pasted into
the source and it stops parsing — reported as `SyntaxError: Unexpected token ':'`.

## save-criteria.js

Paste into the **Scripts → After response** tab of a `DeriveCriteriaFromDescription`
gRPC request. It sets one variable — `criteria` — so the next request can use the derived
criteria, and fails visibly if the response shape is not what it expects. Everything else it
reports goes to the console, so the environment stays to the one thing that is actually
consumed.

Then, in a `ScoreAgainstCriteria` request, interpolate it **without quotes** — the variable
already holds JSON:

```json
{
  "profile": { "candidateId": "cand-7781", "skills": ["Go"] },
  "criteria": {{criteria}},
  "language": "LANGUAGE_ENGLISH"
}
```

Quoting it (`"criteria": "{{criteria}}"`) sends a string where an array belongs and the
request is rejected.

Criterion ids must survive the round trip unchanged. The service requires exactly one
assessment per criterion sent, and no others, so an edited or dropped id fails the whole
scoring call rather than that one criterion.

Postman renders gRPC responses with the `.proto`'s own `snake_case` field names, so the
criteria array comes back in the shape the next request wants. The script reads both
spellings anyway, since Postman has changed this before.
