"""A provider that answers from a script, for demonstrating the service with
no credential and no network.

This exists because a live demo depends on someone else's API being up and on
the room's wifi. It is a rehearsal and fallback path, not a test double —
tests use FakeProvider.

Answers are shaped like real ones so the response is readable, and are marked
as canned in three places that travel with them: the attribution the caller
receives, the proposed title, and the justification. An earlier version
prefixed every field with a banner, which made the output unreadable and hid
the thing a demo is trying to show.
"""

import re
import time
from typing import Any

from ai_service.config import Settings
from ai_service.provider import ProviderAnswer

MODEL = "canned-answers-no-model-was-called"
NOTE = "Canned demo answer — no language model was called."

# How _render_criteria writes a criterion into the prompt.
_CRITERION = re.compile(
    r"^- id=(\S+) \((MUST HAVE|weight (\d+))\)(?: \[[^\]]*\])?: (.*)$", re.MULTILINE
)


class CannedProvider:
    def __init__(self, settings: Settings | None = None) -> None:
        self.settings = settings

    def json_answer(
        self,
        *,
        system: str,
        user: str,
        schema: dict[str, Any],
        schema_name: str,
        schema_description: str,
    ) -> ProviderAnswer:
        builder = {
            "proposed_criteria": self._criteria,
            "candidate_profile": self._profile,
            "criterion_assessments": self._assessments,
            "interview_questions": self._questions,
        }[schema_name]
        # A load test needs the time a model takes, not its answer: holding the
        # worker thread for that long is what makes the queue and the replica
        # count matter (AI_SERVICE_CANNED_LATENCY_S).
        if self.settings is not None and self.settings.canned_latency_s > 0:
            time.sleep(self.settings.canned_latency_s)
        return ProviderAnswer(data=builder(user), model=MODEL)

    def _parsed(self, user: str) -> list[tuple[str, bool, str]]:
        """(id, is a must-have, text) for each criterion the prompt listed."""
        return [
            (m.group(1), m.group(2) == "MUST HAVE", m.group(4)) for m in _CRITERION.finditer(user)
        ]

    def _criteria(self, user: str) -> dict[str, Any]:
        return {
            "proposed_title": "Backend Engineer (canned)",
            "criteria": [
                {
                    "text": "3+ years building production backend services",
                    "must_have": True,
                    "weight": 0,
                    "category": "experience",
                },
                {
                    "text": "Strong SQL and relational data modelling",
                    "must_have": False,
                    "weight": 40,
                    "category": "skills",
                },
                {
                    "text": "Experience operating distributed systems",
                    "must_have": False,
                    "weight": 60,
                    "category": "skills",
                },
            ],
            "uninterpreted": ["Should be a team player"],
        }

    def _profile(self, user: str) -> dict[str, Any]:
        return {
            "work_history": [
                {
                    "title": "Backend Engineer",
                    "organisation": "Acme Payments",
                    "start": "2021-03",
                    "end": "",
                    "summary": "Built and operated payment services in Go on PostgreSQL.",
                }
            ],
            "skills": ["Go", "PostgreSQL", "Kubernetes"],
            "education": [
                {
                    "qualification": "BSc Computer Science",
                    "institution": "Chulalongkorn University",
                    "completed": "2019",
                }
            ],
            "additional_text": f"{NOTE} Extracted from {len(user)} characters of resume text.",
        }

    def _assessments(self, user: str) -> dict[str, Any]:
        # Alternating met/unmet so one run shows both a satisfied criterion and
        # a failed one without needing two prepared resumes.
        criteria = self._parsed(user)
        assessments = []
        for index, (cid, _must, text) in enumerate(criteria):
            met = index % 2 == 0
            assessments.append(
                {
                    "criterion_id": cid,
                    "met": met,
                    "score": 85 if met else 25,
                    "evidence": (
                        f"Resume evidence for “{text}” would be quoted here."
                        if met
                        else f"Nothing in the resume evidences “{text}”."
                    ),
                }
            )
        return {
            "assessments": assessments,
            "justification": (
                f"{NOTE} A real justification would cite what this candidate demonstrably "
                "has and what they are missing against the must-haves."
            ),
        }

    def _questions(self, user: str) -> dict[str, Any]:
        criteria = self._parsed(user)
        kinds = ["resume_claim", "must_have_gap", "role_depth"]
        return {
            "questions": [
                {
                    "text": f"Tell me about your experience with {text.lower()}.",
                    "criterion_id": cid,
                    "source_evidence": f"Criterion “{text}”",
                    "kind": kinds[index % len(kinds)],
                }
                for index, (cid, _must, text) in enumerate(criteria)
            ]
        }
