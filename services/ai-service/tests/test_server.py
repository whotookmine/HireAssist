"""Serving concerns: what the health check says, and what reflection exposes."""

import grpc
import pytest
from grpc_health.v1 import health_pb2, health_pb2_grpc

from ai_service.config import Settings
from ai_service.providers import FakeProvider
from ai_service.server import SERVICE_NAME


@pytest.fixture
def serving():
    """Start a server and return (channel, stop) for a given provider."""
    running = []

    def _start(provider=None, **overrides):
        from ai_service.server import build_server

        config = Settings(port=0, api_key="test-key" if provider else None, **overrides)
        server, port = build_server(config, provider=provider)
        server.start()
        channel = grpc.insecure_channel(f"localhost:{port}")
        running.append((server, channel))
        return channel

    yield _start

    for server, channel in running:
        channel.close()
        server.stop(None)


def test_health_is_serving_when_a_provider_is_configured(serving):
    channel = serving(provider=FakeProvider({}))
    stub = health_pb2_grpc.HealthStub(channel)
    response = stub.Check(health_pb2.HealthCheckRequest(service=SERVICE_NAME))
    assert response.status == health_pb2.HealthCheckResponse.SERVING


def test_health_is_not_serving_without_a_credential(serving):
    """Reachable is not the same as able to work. An instance with no key
    would accept every resume and fail it; health says keep traffic away."""
    channel = serving(provider=None)
    stub = health_pb2_grpc.HealthStub(channel)
    response = stub.Check(health_pb2.HealthCheckRequest(service=SERVICE_NAME))
    assert response.status == health_pb2.HealthCheckResponse.NOT_SERVING


def test_the_empty_service_name_is_answered_too(serving):
    """What a plain `grpc_health_probe` with no -service flag asks for."""
    channel = serving(provider=FakeProvider({}))
    stub = health_pb2_grpc.HealthStub(channel)
    assert (
        stub.Check(health_pb2.HealthCheckRequest()).status == health_pb2.HealthCheckResponse.SERVING
    )


def test_reflection_lists_the_service(serving):
    from grpc_reflection.v1alpha import reflection_pb2, reflection_pb2_grpc

    channel = serving(provider=FakeProvider({}))
    stub = reflection_pb2_grpc.ServerReflectionStub(channel)
    responses = stub.ServerReflectionInfo(
        iter([reflection_pb2.ServerReflectionRequest(list_services="")])
    )
    listed = {s.name for r in responses for s in r.list_services_response.service}
    assert SERVICE_NAME in listed


def test_reflection_can_be_turned_off(serving):
    from grpc_reflection.v1alpha import reflection_pb2, reflection_pb2_grpc

    channel = serving(provider=FakeProvider({}), enable_reflection=False)
    stub = reflection_pb2_grpc.ServerReflectionStub(channel)
    with pytest.raises(grpc.RpcError):
        list(
            stub.ServerReflectionInfo(
                iter([reflection_pb2.ServerReflectionRequest(list_services="")])
            )
        )


def test_the_canned_provider_serves_without_a_credential():
    """The offline demo path: a provider that needs no key, so a rehearsal
    does not depend on someone else's API being up."""
    from ai_service.providers import CannedProvider, provider_for

    provider = provider_for(Settings(provider="canned", api_key=None))
    assert isinstance(provider, CannedProvider)


def test_canned_answers_are_labelled_as_canned():
    """A canned answer on a projector must not be mistakable for a real one."""
    from ai_service.providers.canned import NOTE, CannedProvider

    answer = CannedProvider().json_answer(
        system="",
        user="- id=cr-1 (MUST HAVE): production Go",
        schema={},
        schema_name="criterion_assessments",
        schema_description="",
    )
    # Marked where it travels with the answer, not stamped into every field.
    assert answer.data["justification"].startswith(NOTE)
    assert "no-model-was-called" in answer.model


def test_the_canned_provider_can_stand_in_for_a_models_response_time(monkeypatch):
    """A load test sets a latency so that a worker is held as long as a real
    call would hold it; the demo, which sets none, must not wait at all."""
    from ai_service.providers import canned

    slept = []
    monkeypatch.setattr(canned.time, "sleep", slept.append)
    ask = dict(system="", user="", schema={}, schema_name="interview_questions", schema_description="")

    canned.CannedProvider(Settings(provider="canned", canned_latency_s=6)).json_answer(**ask)
    canned.CannedProvider(Settings(provider="canned")).json_answer(**ask)
    canned.CannedProvider().json_answer(**ask)

    assert slept == [6]


def test_canned_evidence_quotes_the_criterion_it_was_given():
    """The parser must read the criterion text out of the rendered prompt; a
    silent mismatch would make every canned answer generic."""
    from ai_service.providers.canned import CannedProvider

    answer = CannedProvider().json_answer(
        system="",
        user="- id=cr-1 (MUST HAVE) [skills]: production Go\n- id=cr-2 (weight 40): Kafka",
        schema={},
        schema_name="criterion_assessments",
        schema_description="",
    )
    assessments = answer.data["assessments"]
    assert [a["criterion_id"] for a in assessments] == ["cr-1", "cr-2"]
    assert "production Go" in assessments[0]["evidence"]
    assert "Kafka" in assessments[1]["evidence"]
