# ADR-009: Short-lived JWT access tokens with rotating refresh tokens

**Date:** 2026-09-24
**Deciders:** Thanwarat Korcharoenkiat (Identity Service owner), proposed to the team

> In the context of signing members in and telling every service who is calling and with which
> role, facing the need to end a removed member's access quickly without making the Identity
> Service a dependency of every request, we propose short-lived signed access tokens that the
> gateway and any service acting on the role verify themselves, backed by rotating refresh tokens
> held by the Identity Service, to achieve a role that travels with the request and can be checked
> anywhere without a network call, accepting that a removed or demoted member keeps their access
> for up to fifteen minutes and that the signing key becomes the most dangerous secret in the
> system.

---

## Summary

### Issue

Every request must carry a verified identity and role. A member signs in with an email address
and a password and receives a token identifying them and their role (FR-0.1); a request with a
missing, invalid or expired token is rejected (FR-0.2); a request above the caller's role is
rejected and the attempt recorded (FR-0.4). The gateway checks every request. What has not been
decided is what the token is, who checks it, and how the role reaches the services behind the
gateway, which enforce it too.

Four forces shape the answer.

**Removal has to end access.** An Admin removes members and changes roles (FR-6.1). Access to
candidate personal data is a PDPA obligation, so a removed member must stop reaching that data,
and soon — not at some distant expiry.

**The role has to travel to the services.** Enforcement cannot live only at the gateway, so the
services behind it act on the caller's role as well. Passed as a plain header, the role is
whatever the sender claims: any process inside the cluster could send a request claiming to be
an Admin.

**The check runs on every request.** Whatever validates a token sits on the path of every call
in the system. If that is a network call, the service answering it becomes a dependency of work
that has nothing to do with identity, and its outage becomes everyone's.

**Security code is where a small team makes expensive mistakes.** Four students, a deadline, and
a mechanism that must be right the first time favour a standard format with mature libraries
over anything written from scratch.

Scale is not a force. One deployment serves one company, so an installation has tens of members.
Any mechanism considered here is fast enough.

### Decision

**Access is carried by short-lived signed JWTs; sessions are held as rotating refresh tokens in
the Identity Service.**

- **On sign-in** the Identity Service checks the password and issues two tokens.
  - An **access token**: a JWT signed with ES256, valid for fifteen minutes, carrying the member
    id, the role, the time of issue and the expiry. It is sent as a bearer token on every request.
  - A **refresh token**: 256 random bits with no meaning of their own. The Identity Service
    stores only a hash of it, in PostgreSQL.
- **A refresh token is used once.** Exchanging it returns a new access token and a new refresh
  token and retires the old one. A retired refresh token presented again means a copy exists,
  so every token descended from that sign-in is revoked.
- **A session is the chain of refresh tokens that starts at one sign-in.** It ends eight hours
  after sign-in however often it is refreshed, or earlier when revoked. Then the member signs in
  again.
- **Only the Identity Service can sign.** It holds the private key as a secret. The public key is
  not secret and is handed to every verifier as configuration, together with its key id, so no
  service calls the Identity Service to check a token — not per request and not at startup. The
  configuration may hold more than one public key, which is how a key is rotated.
- **Tokens are verified where they are used.** The gateway checks the signature and expiry of
  every access token itself and does not call the Identity Service per request. It forwards the
  token unchanged. A service that acts on the caller's role verifies the forwarded token the same
  way, and no service accepts a role from an unsigned header.
- **The role is read fresh at every refresh**, so a role change reaches the next access token.
  Removing a member deletes their refresh tokens.
- **Member management does not trust the token's role.** The Identity Service re-reads the
  caller's role from its own database before creating, removing or re-roling a member, so a
  demoted Admin loses that power immediately rather than after the window.

The lifetimes and the algorithm are configuration with these defaults. The decision is the shape:
a short-lived signed credential verified locally, and a long-lived one held where it can be
deleted.

### Status

**Proposed.** It asks the gateway to verify tokens itself and the services behind it to verify
the tokens they receive, and both belong to other owners. It becomes Accepted when they agree.

### Group

Security · Communication

---

## Details

### Assumptions

- An installation has tens of members and a few hundred requests a minute at most. Nothing here
  is chosen for throughput.
- Every language a service may be written in has a maintained JWT library that verifies ES256.
  True of every mainstream one; a gate on anything unusual.
- Clocks across services agree within seconds. Expiry checks depend on it; verifiers allow a
  leeway of thirty seconds.
- The web application renews an access token silently and retries a request that failed because
  the token expired. That is where the inconvenience of a fifteen-minute token is absorbed.
- A window of up to fifteen minutes after removal is acceptable to a customer. Shortening it only
  increases refresh traffic, which at this scale costs nothing.
- The signing key changes rarely — when it leaks, or on a schedule measured in months — so
  rotating it by hand is acceptable.

### Constraints

- [ADR-001](ADR-001-service-decomposition.md) makes the gateway validate the session on every
  request; this record decides how, and keeps that check at the gateway.
- [ADR-003](ADR-003-polyglot-persistence.md) places users, sessions, memberships and roles in
  PostgreSQL; the refresh tokens are those sessions.
- [ADR-006](ADR-006-single-tenant-deployment.md) makes a session identify a member and a role,
  not a company, so the token carries no tenant.
- Authentication is required of us alongside the business use cases, and so is an API gateway.

### Positions

1. **Opaque random tokens and a session table**, validated by the Identity Service on every
   request.
2. **Long-lived self-contained JWTs**, valid for a working day, with no state on the server.
3. **Short-lived JWT access tokens with rotating refresh tokens**, verified where they are used
   *(chosen)*.
4. **Short-lived JWT access tokens that the gateway still sends to the Identity Service** to be
   validated on every request.
5. **An external identity provider** — a self-hosted Keycloak or a hosted OpenID Connect service —
   issuing the tokens.

### Argument

**Against opaque tokens (1).** It is the simplest and the only option with immediate revocation,
and at this scale its lookup costs nothing. Two things decide against it. It makes the Identity
Service and its database a dependency of every request, so an Identity outage stops even work
that never touches identity data. And it answers the role question badly: the services behind
the gateway would either call the Identity Service themselves, adding one more per-request
dependency per service, or trust a role header set by the gateway, which anything inside the
cluster can forge.

**Against long-lived JWTs (2).** Verification is local, as in the chosen option, but a token
issued for a working day cannot be withdrawn. A removed member keeps full access for hours, which
contradicts what managing members is for. Fixing that needs a revocation list checked on every
request — option 1 again, with a signing key added.

**Against validating short-lived tokens centrally (4).** It keeps the Identity Service on every
request, the cost of option 1, and still leaves a fifteen-minute window, the cost of option 3.
It has the drawbacks of both and the benefit of neither. It was considered only because the
runtime described so far has the gateway calling the Identity Service for each request.

**Against an external identity provider (5).** It is the most complete — multi-factor sign-in,
single sign-on, implementations audited by others. It is also one more stateful system in every
customer deployment and on every developer's laptop, it moves member data out of the service that
owns it, and the sign-in use case needs none of what it adds: there is no self-service sign-up, no
password reset and no single sign-on. Revisit if a customer requires single sign-on.

**On signing with a key pair.** With a shared secret (HS256) every verifier holds the secret, so
any of them could mint an Admin token. With ES256 only the Identity Service can sign, and the
gateway and services can only check. ES256 rather than RS256 for smaller keys and tokens; both
are supported by every mainstream library and gateway.

**On distributing the public key.** Verifiers could fetch the key from an endpoint on the Identity
Service and cache it, which lets a key be rotated without touching them. That keeps a dependency on
the Identity Service at startup — a service that starts while Identity is down cannot verify
anything — and gives Identity callers other than the gateway. Handing the key out as configuration
removes both, at the price of making rotation a configuration change in every verifier. The key is
expected to change rarely, so the price is paid rarely.

**For the chosen option (3).** Revocation is bounded by the access-token lifetime, because the
long-lived credential stays on the server, where it can be deleted. The Identity Service leaves
the request path. The role reaches every service in a form it can check without trusting the
network. Rotation turns a stolen refresh token into a detectable event instead of a silent one.

### Implications

**What this buys us**

- The Identity Service is off the path of every request. If it is down, members already signed
  in keep working for up to fifteen minutes; only sign-in and refresh stop.
- No service calls the Identity Service to check a token, at startup or per request, so the
  gateway remains its only caller.
- The role reaches every service in a form the service can verify itself. The question of how the
  role travels on internal calls is answered without trusting network position.
- A copy of the Identity database yields no usable refresh token, since only hashes are stored,
  and no password.
- A stolen refresh token is detected the first time both copies are used, and the session is
  revoked.

**What it costs us**

- **Removal and demotion are not immediate.** A removed or demoted member keeps their current
  access token's rights for up to fifteen minutes in every service except member management,
  which re-reads the role. Managing members delivers its outcome only after that window. This is
  the trade-off this record exists to make explicit.
- **The signing key is the most dangerous secret in the system.** Whoever holds it can mint an
  Admin token that every service accepts. It lives in a secret store and never in the repository;
  rotating it or losing it signs everyone out.
- **More moving parts than a session table:** two kinds of token, a refresh flow the web
  application must implement, rotation and reuse rules, a public key to distribute to every
  verifier, and clock agreement between services.
- **Rotating the signing key is a coordinated change.** The new public key goes into every
  verifier's configuration before the Identity Service signs with the new private key, and both
  are accepted until tokens signed with the old one have expired. A verifier that is missed
  rejects every member until it is updated.
- **Verification is implemented in every service that acts on the role**, in each of its
  languages. That duplication was already accepted when languages became a per-service choice; it
  now has a concrete requirement, a maintained JWT library that verifies ES256.
- **An access token cannot be withdrawn on its own.** A stolen access token works until it
  expires.
- **The web application must keep the refresh token where scripts cannot read it**, typically an
  HttpOnly cookie. That is a decision for the web application and the gateway, created by this
  one.
- **The runtime described so far changes:** the gateway stops calling the Identity Service to
  validate each request, and the Identity Service gains a refresh operation.

---

## Related

### Related decisions

- [ADR-001](ADR-001-service-decomposition.md) — the gateway as the point where every request is
  bound to a verified identity and role. This record decides the mechanism and keeps the check
  there, adding verification in the services behind it.
- [ADR-003](ADR-003-polyglot-persistence.md) — sessions in PostgreSQL; the refresh tokens are
  stored there, as hashes.
- [ADR-005](ADR-005-per-service-language.md) — role enforcement implemented in more than one
  service and language; this record gives it one mechanism and adds a JWT library to what each
  language must provide.
- [ADR-006](ADR-006-single-tenant-deployment.md) — a session identifies a member and a role, not a
  company, so the token has no tenant claim.

### Related requirements

- **UC-0, signing in**, and **UC-6, managing members and roles**, are the use cases served.
- **FR-0.1**, authenticating by email and password and issuing a token that identifies the member
  and their role, is the access token.
- **FR-0.2**, rejecting a missing, invalid or expired token, is the signature and expiry check at
  the gateway and in each service.
- **FR-0.3**, two roles with Admin holding every Recruiter permission, is the role claim.
- **FR-0.4**, rejecting a request above the caller's role and recording the attempt, is unchanged:
  the gateway still rejects and records.
- **FR-6.1**, an Admin creating, removing and re-roling members, now takes effect within the
  access-token lifetime, and immediately for member management itself.
- **NFR-09**, availability of 99% in business hours, no longer depends on the Identity Service
  answering every request.
- **NFR-11**, TLS for all data in transit, is what makes a bearer token safe to send.
- **Course requirements:** authentication alongside the business use cases, and an API gateway,
  which is where every token is first verified.

### Related artifacts

The Identity Service's REST contract; the Service–Operations–Collaborators table and the runtime
view, which currently show the gateway asking the Identity Service to validate every request; and
the sign-in use case's alternate flow for an expired token, since an expired access token no
longer ends the session.

### Related principles

- *PDPA is a design force, not a feature* — the fifteen-minute window is a privacy trade-off made
  openly, with its size under configuration.
- *Easily reversible* — returning to opaque tokens changes the token format and the gateway's
  check, not the operations the Identity Service offers.

---

## Notes

The runtime described before this record assumed option 1: the gateway calls the Identity Service
for every request and the Identity Service keeps a session table. This record departs from that
on purpose. The reasons are availability and the way the role travels to the services, not
performance — at tens of members, option 1 would be fast enough.

It is left *Proposed* because two of its commitments fall on services other than the Identity
Service: the gateway must verify tokens itself, and a service that acts on the role must verify
the token it receives.
