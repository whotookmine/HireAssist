import { screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { db } from '../mocks/db'
import { renderSignedIn } from './render'

describe('managing members and roles (UC-6)', () => {
  it('adds a member with a temporary password (FR-6.1, FR-6.5)', async () => {
    const { user } = await renderSignedIn('/members', 'admin')
    await user.click(await screen.findByRole('button', { name: 'Add member' }))
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getByLabelText(/Email/), 'ploy@acme.example')
    await user.type(within(dialog).getByLabelText(/Temporary password/), 'a-temporary-password-1')
    await user.click(within(dialog).getByRole('button', { name: 'Add member' }))

    expect(await screen.findByText('ploy@acme.example')).toBeInTheDocument()
    const row = screen.getByText('ploy@acme.example').closest('tr')!
    expect(within(row).getByText('Temporary password')).toBeInTheDocument()
  })

  it('refuses to remove the last Admin (FR-6.3)', async () => {
    const { user } = await renderSignedIn('/members', 'admin')
    await user.click(await screen.findByRole('button', { name: 'Remove nok@acme.example' }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Remove' }))
    expect(await screen.findByText('this would leave no active admin')).toBeInTheDocument()
    expect(db.members.find((m) => m.email === 'nok@acme.example')!.removedAt).toBeNull()
  })

  it('removes a member, who can then no longer sign in (FR-6.2)', async () => {
    const { user } = await renderSignedIn('/members', 'admin')
    await user.click(await screen.findByRole('button', { name: 'Remove somchai@acme.example' }))
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Remove' }))
    expect(await screen.findByText('somchai@acme.example was removed.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Remove somchai@acme.example' })).not.toBeInTheDocument()
    // Kept, so past records still name them.
    expect(db.members.find((m) => m.email === 'somchai@acme.example')!.removedAt).not.toBeNull()
  })

  it('keeps a Recruiter off the page', async () => {
    await renderSignedIn('/members')
    expect(await screen.findByRole('heading', { name: 'Job openings' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Members' })).not.toBeInTheDocument()
  })
})
