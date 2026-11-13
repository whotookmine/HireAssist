import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { Role } from '../api/types'
import { App } from '../App'
import { db } from '../mocks/db'

/** Renders the whole app at a path, in English. */
export function renderApp(path = '/') {
  window.history.pushState({}, '', path)
  const user = userEvent.setup()
  return { user, ...render(<App language="en" />) }
}

/** Renders the app already signed in as a demo member — the recruiter unless asked — through the refresh cookie (ADR-009). */
export async function renderSignedIn(path: string, role: Role = 'recruiter') {
  const member = db.members.find((m) => m.role === role)!
  db.refreshTokens.set('rt_test', member.id)
  db.refreshCookie = 'rt_test'
  const result = renderApp(path)
  await screen.findByText(member.email)
  return result
}
