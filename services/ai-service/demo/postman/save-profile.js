// Postman "After response" script for ExtractProfileFromText.
//
// Saves the extracted candidate profile so ScoreAgainstCriteria and
// GenerateInterviewQuestions can use it without anyone retyping a resume.
// Sets one variable: `profile`, holding the profile as JSON. Interpolate it
// UNQUOTED (no surrounding quotes) -- see README.md.
//
// NOTHING IN THIS FILE MAY CONTAIN A VARIABLE REFERENCE IN DOUBLED BRACES,
// not even inside a comment or a string. Postman substitutes those before it
// evaluates the script, so a saved JSON value gets pasted into the source and
// the script stops parsing -- which reads as "Unexpected token ':'".
//
// Plain ES5 throughout, to keep the sandbox happy.

// Postman has moved the gRPC response accessor between versions, and the
// shape differs between unary and streaming calls, so search the response
// object rather than naming a property.
function findProfile(root, maxDepth) {
    var seen = [];
    var foundAt = null;

    function walk(node, depth, path) {
        if (!node || typeof node !== "object" || depth > maxDepth) { return null; }
        for (var i = 0; i < seen.length; i++) {
            if (seen[i] === node) { return null; }
        }
        seen.push(node);

        if (node.profile && typeof node.profile === "object") {
            foundAt = path;
            return node.profile;
        }
        var keys = Object.keys(node);
        for (var k = 0; k < keys.length; k++) {
            var value;
            try {
                value = node[keys[k]];      // a getter may throw; skip it
            } catch (err) {
                continue;
            }
            var hit = walk(value, depth + 1, path + "." + keys[k]);
            if (hit) { return hit; }
        }
        return null;
    }

    var found = walk(root, 0, "pm.response");
    if (!found) {
        try { found = walk(pm.response.json(), 0, "pm.response.json()"); } catch (e1) {}
    }
    if (!found) {
        try {
            found = walk(pm.response.messages.toArray(), 0, "pm.response.messages.toArray()");
        } catch (e2) {}
    }
    if (found) { console.log("profile found at " + foundAt); }
    return found;
}

var profile = findProfile(pm.response, 6);

if (!profile) {
    console.log("--- could not locate the profile. pm.response looks like: ---");
    try {
        console.log("own keys: " + JSON.stringify(Object.keys(pm.response)));
        var proto = Object.getPrototypeOf(pm.response) || {};
        console.log("methods: " + JSON.stringify(Object.getOwnPropertyNames(proto)));
    } catch (e3) {
        console.log("shape not readable");
    }
    var probes = ["data", "body", "stream", "messages"];
    for (var p = 0; p < probes.length; p++) {
        try {
            console.log(probes[p] + ": " + JSON.stringify(pm.response[probes[p]]).slice(0, 300));
        } catch (e4) {
            console.log(probes[p] + ": not readable");
        }
    }
}

pm.test("the response carries a profile", function () {
    pm.expect(profile, "no property of pm.response held a profile object").to.not.be.null;
});

if (profile) {
    var id = profile.candidate_id || profile.candidateId;

    pm.test("the profile keeps the candidate id that was sent", function () {
        pm.expect(id, "candidate_id").to.be.a("string");
        pm.expect(id.length, "candidate_id is empty").to.be.above(0);
    });

    // The profile has no field for contact details, so the structured slots
    // cannot carry them -- but additional_text could, if the prompt were
    // ignored. Extraction is the only call that sees the whole resume, so
    // this is the cheapest check that what it dropped stayed dropped.
    var asText = JSON.stringify(profile);

    pm.test("no email address reached the profile", function () {
        var hits = asText.match(/[\w.+-]+@[\w-]+\.[\w.]+/g) || [];
        pm.expect(hits, "email addresses found: " + hits.join(", ")).to.be.empty;
    });

    pm.test("no phone number reached the profile", function () {
        // Seven or more digits in a row, or a dashed run of nine or more.
        // A year, or a date like 2021-03, is shorter than both.
        var hits = asText.match(/\d{7,}|(?:\d[\s-]?){9,}/g) || [];
        pm.expect(hits, "number sequences found: " + hits.join(", ")).to.be.empty;
    });

    pm.environment.set("profile", JSON.stringify(profile, null, 2));

    var jobs = profile.work_history || profile.workHistory || [];
    var education = profile.education || [];
    var skills = profile.skills || [];

    console.log("saved profile " + id + " to the profile variable");
    for (var j = 0; j < jobs.length; j++) {
        console.log("  " + jobs[j].title + " @ " + jobs[j].organisation +
            "  (" + jobs[j].start + " - " + (jobs[j].end || "present") + ")");
    }
    console.log("  skills: " + (skills.length > 0 ? skills.join(", ") : "(none)"));
    for (var e = 0; e < education.length; e++) {
        console.log("  " + education[e].qualification + ", " +
            education[e].institution + " " + education[e].completed);
    }
    var extra = profile.additional_text || profile.additionalText || "";
    console.log("  additional_text: " + (extra ? extra : "(empty)"));
    console.log("Check by eye: the candidate's name must not appear above. " +
        "A name is not matchable by pattern, so no test covers it.");
}
