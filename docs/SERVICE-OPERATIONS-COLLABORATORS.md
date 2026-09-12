# Service–Operations–Collaborators

HireAssist · service decomposition per **ADR-001** — 5 services + 1 API Gateway.
Protocol of each collaboration is marked **(REST)**, **(gRPC)** or **(MQ)** for RabbitMQ.

| Service | Operations | Collaborators |
|---|---|---|
| **Identity & Workspace Service**<br><br>*owns: accounts, workspaces,<br>memberships, roles, sessions*<br><br>UC-0 · UC-6 | `authenticateMember()`<br>`validateSession()`<br>`getMemberRole()`<br>`inviteMember()`<br>`removeMember()`<br>`changeMemberRole()` | **Compliance & Insights Service** *(MQ)*<br>　publishes `AuthorisationRejected` |
| **Hiring Service**<br><br>*owns: job openings, criteria,<br>screening batches, screening results,<br>decisions, interview guides*<br><br>UC-1 · UC-2 · UC-3 | `createJobOpening()`<br>`reviseScreeningCriteria()`<br>`getScreeningCriteria()`<br>`getJobOpening()`<br>`pauseOpening()`<br>`resumeOpening()`<br>`closeOpening()`<br>`submitScreeningBatch()`<br>`getBatchStatus()`<br>`getRankedResults()`<br>`overrideScore()`<br>`recordDecision()`<br>`generateInterviewGuide()`<br>`reviseGuide()`<br>`getInterviewGuide()`<br>`recordInterviewNote()`<br>`eraseHiringData()` | **Object Storage Adapter** *(REST)*<br>　`storeResumeFile()`<br>**Resume Processing Service** *(MQ)*<br>　publishes `ResumeSubmitted`<br>　consumes `ResumeScored`<br>**AI Service** *(gRPC)*<br>　`deriveCriteriaFromDescription()`<br>　`generateInterviewQuestions()`<br>**Compliance & Insights Service** *(MQ)*<br>　publishes pipeline domain events<br>**Email Adapter** *(REST)*<br>　`sendEmail()` |
| **Resume Processing Service**<br><br>*owns: candidate profiles,<br>parsed resume text, consent records*<br><br>UC-2 (per-resume work) | *consumes `ResumeSubmitted`*<br>`getCandidateProfile()`<br>`listCandidateProfiles()`<br>`recordConsent()`<br>`listUnparsableResumes()`<br>`eraseCandidateProfile()` | **Object Storage Adapter** *(REST)*<br>　`fetchResumeFile()`<br>**Document Parsing Adapter** *(REST)*<br>　`extractResumeText()`<br>**AI Service** *(gRPC)*<br>　`extractProfileFields()`<br>　`scoreAgainstCriteria()`<br>**Hiring Service** *(REST)*<br>　`getScreeningCriteria()`<br>**Hiring Service** *(MQ)*<br>　publishes `ResumeScored` |
| **AI Service**<br><br>*owns: prompts and model credentials<br>— no domain data*<br><br>UC-1 · UC-2 · UC-3 | `deriveCriteriaFromDescription()`<br>`extractProfileFields()`<br>`scoreAgainstCriteria()`<br>`generateInterviewQuestions()` | **LLM Adapter** *(REST)*<br>　`complete()` |
| **Compliance & Insights Service**<br><br>*owns: retention policies,<br>pipeline metrics, erasure audit log*<br><br>UC-4 · UC-5 | `setRetentionPolicy()`<br>`evaluateRetention()`<br>`renewConsent()`<br>`eraseCandidate()`<br>`getErasureAudit()`<br>`getPipelineDashboard()`<br>`evaluateStaleness()` | **Hiring Service** *(REST)*<br>　`eraseHiringData()`<br>　`getJobOpening()`<br>**Resume Processing Service** *(REST)*<br>　`eraseCandidateProfile()`<br>　`listCandidateProfiles()`<br>　`recordConsent()`<br>**Email Adapter** *(REST)*<br>　`sendEmail()` |

**API Gateway** is the entry point, not a domain service. It terminates TLS, routes, rate-limits,
resolves the workspace, and validates the session by calling
`Identity & Workspace Service.validateSession()`. It owns no data.

**External systems**, each behind an adapter: Object / Blob Storage, Document Parsing / OCR Provider,
LLM Provider (reached only by AI Service), Email Provider.

No service reads another service's data store. Every crossing is an operation above or a RabbitMQ event.
