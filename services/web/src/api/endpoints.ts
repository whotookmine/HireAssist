// One function per Gateway call. Each names the operation it reaches in the
// Service–Operations–Collaborators table, so a screen can be traced to the service behind it.

import { api, ApiError, COOKIE_TRANSPORT } from './client'
import type {
  AccessRejection,
  Criterion,
  Erasure,
  ErasureStatus,
  HeldExpiry,
  HoldDecision,
  Member,
  PipelineOpening,
  RetentionPolicy,
  Role,
  InterviewGuide,
  JobOpening,
  JobOpeningDraft,
  JobOpeningProposal,
  QuestionDraft,
  Outcome,
  RankedResults,
  ScreenedCandidate,
  ScreeningBatch,
  ScreeningResult,
  Session,
  SessionMember,
} from './types'

// ── Auth → Identity ───────────────────────────────────────────────────────────────────────

/** Identity.authenticateMember(). The refresh token comes back as an HttpOnly cookie, not in the body. */
export const signIn = (email: string, password: string) =>
  api<Session>('/auth/sign-in', { method: 'POST', json: { email, password }, anonymous: true, headers: COOKIE_TRANSPORT })

/** The gateway's own route: clears the refresh cookie, which a script cannot do. */
export const signOut = () => api<void>('/auth/sign-out', { method: 'POST', anonymous: true })

/** Identity.changePassword() — takes credentials, not a token: a temporary password cannot sign in (FR-0.5). */
export const changePassword = (email: string, currentPassword: string, newPassword: string) =>
  api<SessionMember>('/auth/change-password', {
    method: 'POST',
    json: { email, current_password: currentPassword, new_password: newPassword },
    anonymous: true,
  })

// ── UC-1 → Hiring ─────────────────────────────────────────────────────────────────────────

/** Hiring.listJobOpenings() */
export const listJobOpenings = () => api<JobOpening[]>('/job-openings')

/** Hiring.getJobOpening() */
export const getJobOpening = (id: string) => api<JobOpening>(`/job-openings/${id}`)

/** Hiring.proposeJobOpening() — the Hiring Service asks the AI Service; nothing is stored. */
export const proposeJobOpening = (description: string) =>
  api<JobOpeningProposal>('/job-openings/propose', { method: 'POST', json: { description } })

/** Hiring.createJobOpening() */
export const createJobOpening = (draft: JobOpeningDraft) =>
  api<JobOpening>('/job-openings', { method: 'POST', json: draft })

/** Hiring.reviseScreeningCriteria() */
export const reviseScreeningCriteria = (id: string, criteria: Criterion[]) =>
  api<JobOpening>(`/job-openings/${id}/criteria`, { method: 'PUT', json: criteria })

/** Hiring.pauseOpening() */
export const pauseOpening = (id: string) =>
  api<JobOpening>(`/job-openings/${id}/pause`, { method: 'POST' })

/** Hiring.resumeOpening() */
export const resumeOpening = (id: string) =>
  api<JobOpening>(`/job-openings/${id}/resume`, { method: 'POST' })

/** Hiring.closeOpening() */
export const closeOpening = (id: string, outcome: Outcome) =>
  api<JobOpening>(`/job-openings/${id}/close`, { method: 'POST', json: { outcome } })

// ── UC-2 → Hiring ─────────────────────────────────────────────────────────────────────────

/** Hiring.submitScreeningBatch() — answers 202 as soon as the batch is recorded (FR-2.2). */
export const submitScreeningBatch = (jobOpeningId: string, files: File[]) => {
  const form = new FormData()
  for (const file of files) form.append('resumes', file)
  return api<ScreeningBatch>(`/job-openings/${jobOpeningId}/batches`, { method: 'POST', body: form })
}

/** Hiring.getBatchStatus(). There is no operation that lists an opening's batches. */
export const getBatch = (jobOpeningId: string, batchId: string) =>
  api<ScreeningBatch>(`/job-openings/${jobOpeningId}/batches/${batchId}`)

export interface ResultsFilter {
  minScore?: number
  /** A must-have's name: screening reports only which must-haves a resume missed. */
  criterion?: string
}

/** Hiring.getRankedResults() — qualified first, then by score; unreadable resumes apart (FR-2.6, FR-2.7). */
export const getRankedResults = (jobOpeningId: string, filter: ResultsFilter = {}) => {
  const query = new URLSearchParams()
  if (filter.minScore) query.set('min_score', String(filter.minScore))
  if (filter.criterion) query.set('criterion', filter.criterion)
  const qs = query.toString()
  return api<RankedResults>(`/job-openings/${jobOpeningId}/results${qs ? `?${qs}` : ''}`)
}

const resultPath = (jobOpeningId: string, resumeId: string) => `/job-openings/${jobOpeningId}/results/${resumeId}`

/** Hiring.getScreeningResult() — the record, with the justification read from Resume Processing. */
export const getScreeningResult = (jobOpeningId: string, resumeId: string) =>
  api<ScreenedCandidate>(resultPath(jobOpeningId, resumeId))

/** Hiring.overrideScore() — the original score is kept (FR-2.8). */
export const overrideScore = (jobOpeningId: string, resumeId: string, score: number) =>
  api<ScreeningResult>(`${resultPath(jobOpeningId, resumeId)}/override`, { method: 'POST', json: { score } })

/** Hiring.shortlistCandidate() */
export const shortlistCandidate = (jobOpeningId: string, resumeId: string) =>
  api<ScreeningResult>(`${resultPath(jobOpeningId, resumeId)}/shortlist`, { method: 'POST' })

// ── UC-3 → Hiring ─────────────────────────────────────────────────────────────────────────

const guidePath = (jobOpeningId: string, candidateId: string) =>
  `/job-openings/${jobOpeningId}/candidates/${candidateId}/interview-guide`

/** Hiring.getInterviewGuide() */
export const getInterviewGuide = (jobOpeningId: string, candidateId: string) =>
  api<InterviewGuide>(guidePath(jobOpeningId, candidateId))

/** Hiring.generateInterviewGuide() — 409 when the candidate already has one. */
export const generateInterviewGuide = (jobOpeningId: string, candidateId: string, emphasis = '') =>
  api<InterviewGuide>(guidePath(jobOpeningId, candidateId), { method: 'POST', json: emphasis ? { emphasis } : {} })

/** Hiring.reviseGuide(), asking the model again: every question is replaced, notes too (FR-3.4). */
export const regenerateInterviewGuide = (jobOpeningId: string, candidateId: string, emphasis = '') =>
  api<InterviewGuide>(`${guidePath(jobOpeningId, candidateId)}/regenerate`, {
    method: 'POST',
    json: emphasis ? { emphasis } : {},
  })

/**
 * Hiring.reviseGuide(), the Recruiter's own edit: the whole set is sent, so adding, rewording,
 * reordering and removing are one request. A note stays with a question whose text is unchanged.
 */
export const reviseGuideQuestions = (jobOpeningId: string, candidateId: string, questions: QuestionDraft[]) =>
  api<InterviewGuide>(`${guidePath(jobOpeningId, candidateId)}/questions`, { method: 'PUT', json: questions })

/** Hiring.recordInterviewNote() (FR-3.5) */
export const recordInterviewNote = (
  jobOpeningId: string,
  candidateId: string,
  questionId: string,
  note: string,
) =>
  api<InterviewGuide>(`${guidePath(jobOpeningId, candidateId)}/questions/${questionId}/note`, {
    method: 'PUT',
    json: { note },
  })

// ── UC-4 → Compliance & Insights ──────────────────────────────────────────────────────────

/** Compliance.getPipelineDashboard() (FR-4.1) */
export const getPipelineDashboard = () =>
  api<{ openings: PipelineOpening[] }>('/compliance/dashboard').then((d) => d.openings)

/** Compliance.evaluateStaleness() now, rather than at the next scheduled sweep. Admin only. */
export const runStalenessSweep = () =>
  api<{ flagged: string[]; cleared: string[] }>('/compliance/sweeps/staleness', { method: 'POST' })

// ── UC-5 → Compliance & Insights ──────────────────────────────────────────────────────────

/** Compliance's retention policy; null until an Admin sets one (the service answers 404). */
export const getRetentionPolicy = () =>
  api<RetentionPolicy>('/compliance/retention-policy').catch((err) => {
    if (err instanceof ApiError && err.status === 404 && !err.notServed) return null
    throw err
  })

/** Compliance.setRetentionPolicy() (FR-5.1). Admin only. */
export const setRetentionPolicy = (policy: RetentionPolicy) =>
  api<RetentionPolicy>('/compliance/retention-policy', { method: 'PUT', json: policy })

/** Compliance.listHeldExpiries() (FR-5.6) */
export const listHeldExpiries = () =>
  api<{ held_expiries: HeldExpiry[] }>('/compliance/held-expiries').then((d) => d.held_expiries)

/** Compliance.resolveHeldExpiry(): "erase" answers with the erasure, "keep" with the decision. */
export const resolveHeldExpiry = (candidateId: string, decision: HoldDecision) =>
  api<Erasure | { decision: HoldDecision }>(`/compliance/held-expiries/${encodeURIComponent(candidateId)}`, {
    method: 'POST',
    json: { decision },
  })

/** Compliance.eraseCandidate() now (FR-5.7). Admin only. Pending until every holder confirms. */
export const eraseCandidate = (candidateId: string) =>
  api<Erasure>('/compliance/erasures', { method: 'POST', json: { candidate_id: candidateId } })

/** Compliance.getErasureAudit() (FR-5.5). Admin only. */
export const listErasures = (status?: ErasureStatus) =>
  api<{ erasures: Erasure[] }>(`/compliance/erasures${status ? `?status=${status}` : ''}`).then((d) => d.erasures)

/** Compliance.evaluateRetention() now. Admin only. */
export const runRetentionSweep = () =>
  api<{ policy_set: boolean; evaluated: number; warned: string[]; erasures: string[]; held: string[] }>(
    '/compliance/sweeps/retention',
    { method: 'POST' },
  )

/** The access audit log: requests refused for want of a role (FR-0.4). Admin only. */
export const listAccessRejections = () =>
  api<{ rejections: AccessRejection[] }>('/compliance/access-audit').then((d) => d.rejections)

// ── UC-6 → Identity ───────────────────────────────────────────────────────────────────────

/** Identity.listMembers(): active members only (FR-6.5). Admin only. */
export const listMembers = () => api<Member[]>('/members')

/** Identity.createMember(), with a temporary password the member must replace (FR-6.1, FR-0.5). */
export const createMember = (email: string, password: string, role: Role) =>
  api<Member>('/members', { method: 'POST', json: { email, password, role } })

/** Identity.changeMemberRole() — refused when it would leave no active Admin (FR-6.3). */
export const changeMemberRole = (id: string, role: Role) =>
  api<Member>(`/members/${id}`, { method: 'PATCH', json: { role } })

/** Identity.setMemberPassword(): a new temporary password, ending the member's sessions (FR-6.4). */
export const setMemberPassword = (id: string, password: string) =>
  api<Member>(`/members/${id}/password`, { method: 'PUT', json: { password } })

/** Identity.removeMember() — refused for the last active Admin (FR-6.3). */
export const removeMember = (id: string) => api<void>(`/members/${id}`, { method: 'DELETE' })
