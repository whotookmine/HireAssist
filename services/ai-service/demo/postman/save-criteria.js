// Postman "After response" script for DeriveCriteriaFromDescription.
//
// Saves the derived criteria into environment variables so the next request —
// ScoreAgainstCriteria or GenerateInterviewQuestions — can use them without
// anyone retyping an id. Criterion ids must match exactly: the service
// rejects a scoring answer whose assessments do not line up one-for-one with
// the criteria sent.
//
// Sets one variable: `criteria`, the criteria array as JSON. Interpolate it
// UNQUOTED (no surrounding quotes) because it already holds JSON. This file
// must never contain a doubled-brace variable reference, even in a comment:
// Postman substitutes those before evaluating, which injects the saved JSON
// into the source and breaks the script.
//
// Postman has moved the gRPC response accessor between versions and the
// shapes differ between unary and streaming calls, so rather than name one
// property this searches the response object for whatever holds the answer.

function findPayload(root, maxDepth) {
    const seen = new WeakSet();
    const trail = [];

    function walk(node, depth, path) {
        if (!node || typeof node !== "object" || depth > maxDepth) return null;
        if (seen.has(node)) return null;
        seen.add(node);

        if (Array.isArray(node.criteria) && node.criteria.length) {
            trail.push(path || "pm.response");
            return node;
        }
        for (const key of Object.keys(node)) {
            let value;
            try {
                value = node[key];          // a getter may throw; skip it
            } catch (e) {
                continue;
            }
            const hit = walk(value, depth + 1, `${path}.${key}`);
            if (hit) return hit;
        }
        return null;
    }

    // Start from the response itself, then from the accessors that return a
    // fresh object rather than a property.
    let found = walk(root, 0, "pm.response");
    if (found) {
        console.log(`criteria found at ${trail[0]}`);
        return found;
    }
    for (const [name, get] of [
        ["pm.response.json()", () => pm.response.json()],
        ["pm.response.messages.toArray()", () => pm.response.messages.toArray()],
    ]) {
        try {
            found = walk(get(), 0, name);
            if (found) {
                console.log(`criteria found at ${trail[0]}`);
                return found;
            }
        } catch (e) {
            /* accessor not available in this Postman build */
        }
    }
    return null;
}

const body = findPayload(pm.response, 6);

if (!body) {
    // Print the shape so the next run can be fixed in one line.
    console.log("--- could not locate the criteria. pm.response looks like: ---");
    try {
        console.log("own keys:", JSON.stringify(Object.keys(pm.response)));
    } catch (e) {
        console.log("own keys: unavailable");
    }
    try {
        const proto = Object.getPrototypeOf(pm.response) || {};
        console.log("methods:", JSON.stringify(Object.getOwnPropertyNames(proto)));
    } catch (e) {
        console.log("methods: unavailable");
    }
    for (const name of ["data", "body", "stream", "messages"]) {
        try {
            console.log(`${name}:`, JSON.stringify(pm.response[name]).slice(0, 300));
        } catch (e) {
            console.log(`${name}: not readable`);
        }
    }
}

pm.test("the response carries criteria", function () {
    pm.expect(body, "no property of pm.response held a non-empty criteria array").to.not.be.null;
    pm.expect(body.criteria, "criteria").to.be.an("array").that.is.not.empty;
});

if (body && Array.isArray(body.criteria) && body.criteria.length) {
    // Every criterion needs an id: scoring matches on it, and a missing one
    // fails the whole call rather than that criterion.
    pm.test("every criterion has an id", function () {
        const missing = body.criteria.filter((c) => !c.id);
        pm.expect(missing, `criteria with no id: ${JSON.stringify(missing)}`).to.be.empty;
    });

    pm.environment.set("criteria", JSON.stringify(body.criteria, null, 2));

    // Everything below is console output, not state. The title is logged
    // rather than saved because nothing consumes it yet.
    console.log(`saved ${body.criteria.length} criteria to the criteria variable`);
    console.log(`proposed title: ${body.proposed_title || body.proposedTitle || "(none)"}`);
    let weighted = 0;
    for (const c of body.criteria) {
        const must = c.must_have || c.mustHave;
        const weight = c.weight || 0;
        if (!must) weighted += weight;
        console.log(`  ${c.id}  [${must ? "MUST HAVE" : `weight ${weight}`}]  ${c.text}`);
    }

    // The weighted criteria are the only ones that move the score; must-haves
    // gate instead. Worth seeing, because it is easy to end up with the whole
    // score resting on one or two criteria.
    console.log(`weighted total available: ${weighted} across ` +
        `${body.criteria.filter((c) => !(c.must_have || c.mustHave)).length} criteria`);

    const left = body.uninterpreted || [];
    console.log(left.length
        ? `uninterpreted (${left.length}): ${left.join(" | ")}`
        : "uninterpreted: none — check nothing untestable became a criterion");
}
