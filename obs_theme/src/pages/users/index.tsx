import { useState, useEffect, useMemo } from 'react'
import {
  Box, Typography, Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Paper,
  Button, TextField, Dialog, DialogTitle, DialogContent, DialogActions, Chip,
} from '@mui/material'
import { useTheme } from '@mui/material/styles'
import { Search, UserPlus, UserMinus, Key, Copy, Check, Eye, EyeOff } from 'lucide-react'
import { notify } from '../../lib/toast'
import { listUsers, createUser, deleteUser, generateAPIKey, revokeAPIKey, type User } from '../../api/adminApi'
import PageSkeleton from '../../components/common/PageSkeleton'

export default function Users() {
  const theme = useTheme()
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [search, setSearch] = useState('')
  const [createOpen, setCreateOpen] = useState(false)
  const [confirmRemove, setConfirmRemove] = useState<User | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [newUser, setNewUser] = useState({ username: '', password: '' })
  const [createError, setCreateError] = useState('')
  // API key state
  const [keyDialogOpen, setKeyDialogOpen] = useState(false)
  const [generatedKey, setGeneratedKey] = useState('')
  const [generatedUser, setGeneratedUser] = useState('')
  const [keyVisible, setKeyVisible] = useState(false)
  const [copied, setCopied] = useState(false)

  const fetchUsers = async () => {
    try {
      setLoading(true)
      setError('')
      setUsers(await listUsers())
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load users')
      notify.error(err instanceof Error ? err.message : 'Failed to load users')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void fetchUsers() }, [])

  const filteredUsers = useMemo(() => users.filter((u) =>
    u.username.toLowerCase().includes(search.toLowerCase())
  ), [users, search])

  const handleCreate = async () => {
    if (!newUser.username || !newUser.password) {
      setCreateError('Username and password are required')
      return
    }
    setSubmitting(true)
    setCreateError('')
    try {
      await createUser({ username: newUser.username, password: newUser.password })
      notify.success(`User ${newUser.username} created`)
      setCreateOpen(false)
      setNewUser({ username: '', password: '' })
      void fetchUsers()
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : 'Create failed')
    } finally {
      setSubmitting(false)
    }
  }

  const handleDelete = async () => {
    if (!confirmRemove) return
    try {
      await deleteUser(confirmRemove.id)
      notify.success(`User ${confirmRemove.username} removed`)
      setConfirmRemove(null)
      void fetchUsers()
    } catch (err) {
      notify.error(err instanceof Error ? err.message : 'Delete failed')
    }
  }

  const handleGenerateKey = async (userId: string) => {
    try {
      const result = await generateAPIKey(userId)
      const user = users.find((u) => u.id === userId)
      setGeneratedKey(result.api_key)
      setGeneratedUser(user?.username ?? userId)
      setKeyVisible(false)
      setCopied(false)
      setKeyDialogOpen(true)
    } catch (err) {
      notify.error(err instanceof Error ? err.message : 'Failed to generate key')
    }
  }

  const handleRevokeKey = async (userId: string) => {
    const user = users.find((u) => u.id === userId)
    if (!window.confirm(`Revoke API key for ${user?.username ?? userId}?`)) return
    try {
      await revokeAPIKey(userId)
      notify.success(`API key revoked for ${user?.username ?? userId}`)
    } catch (err) {
      notify.error(err instanceof Error ? err.message : 'Revoke failed')
    }
  }

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(generatedKey)
      setCopied(true)
      notify.success('API key copied to clipboard')
      setTimeout(() => setCopied(false), 2000)
    } catch {
      notify.error('Failed to copy')
    }
  }

  if (loading) return <PageSkeleton />

  const headSx = { color: 'text.secondary', fontSize: '11px', fontWeight: 500, letterSpacing: '0.04em', textTransform: 'uppercase' as const, borderColor: 'divider', py: 1 }
  const cellSx = { fontSize: 13, color: 'text.primary', borderColor: 'divider', py: '7px' }

  return (
    <Box sx={{ p: 3 }}>
      <Typography variant="h2" sx={{ mb: 1 }}>Users</Typography>
      <Typography variant="caption2" sx={{ color: 'text.secondary', mb: 3, display: 'block' }}>
        {users.length} user{users.length !== 1 ? 's' : ''}
      </Typography>

      {error && (
        <Box sx={{ mb: 2, display: 'flex', gap: 1, alignItems: 'center' }}>
          <Typography variant="body2" color="error">{error}</Typography>
          <Button size="small" variant="outlined" onClick={fetchUsers}>Retry</Button>
        </Box>
      )}

      <Box sx={{ display: 'flex', gap: 1, mb: 2, alignItems: 'center', flexWrap: 'wrap' }}>
        <TextField size="small" placeholder="Search users..." value={search} onChange={(e) => setSearch(e.target.value)}
          slotProps={{ input: { startAdornment: <Search size={14} color="#8b93a8" style={{ marginRight: 6 }} /> } }} sx={{ width: 260 }} />
        <Box sx={{ flex: 1 }} />
        <Button variant="contained" size="small" startIcon={<UserPlus size={14} />} onClick={() => setCreateOpen(true)} sx={{ fontSize: 13 }}>
          Add User
        </Button>
      </Box>

      <TableContainer component={Paper} sx={{ background: 'transparent', boxShadow: 'none' }}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell sx={headSx}>Username</TableCell>
              <TableCell sx={headSx}>Role</TableCell>
              <TableCell sx={headSx}>Created</TableCell>
              <TableCell sx={headSx}>Updated</TableCell>
              <TableCell align="right" sx={headSx}>Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {filteredUsers.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} sx={{ ...cellSx, textAlign: 'center', color: 'text.secondary', py: 3 }}>
                  {search ? 'No users match your search' : 'No users found'}
                </TableCell>
              </TableRow>
            ) : (
              filteredUsers.map((u) => (
                <TableRow key={u.id} hover sx={{ height: 36 }}>
                  <TableCell sx={cellSx}>
                    <Typography variant="body2" sx={{ fontWeight: 500 }}>{u.username}</Typography>
                  </TableCell>
                  <TableCell sx={cellSx}>
                    <Chip label={u.role} size="small" color={u.role === 'admin' ? 'primary' : 'default'}
                      sx={{ borderRadius: '3px', fontSize: 11, fontWeight: 500 }} />
                  </TableCell>
                  <TableCell sx={{ ...cellSx, color: 'text.secondary', fontSize: 12 }}>
                    {new Date(u.created_at).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric', hour: '2-digit', minute: '2-digit' })}
                  </TableCell>
                  <TableCell sx={{ ...cellSx, color: 'text.secondary', fontSize: 12 }}>
                    {new Date(u.updated_at).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric', hour: '2-digit', minute: '2-digit' })}
                  </TableCell>
                  <TableCell align="right" sx={cellSx}>
                    <Box sx={{ display: 'flex', gap: 0.5, justifyContent: 'flex-end' }}>
                      <Button size="small" variant="outlined" startIcon={<Key size={12} />}
                        onClick={() => handleGenerateKey(u.id)} sx={{ fontSize: 11, height: 28, minWidth: 0, px: 1, gap: 0.5 }}>
                        Key
                      </Button>
                      <Button size="small" variant="outlined" color="error" startIcon={<UserMinus size={12} />}
                        onClick={() => handleRevokeKey(u.id)} sx={{ fontSize: 11, height: 28, minWidth: 0, px: 1, gap: 0.5 }}>
                        Revoke
                      </Button>
                      <Button size="small" variant="outlined" color="error" startIcon={<UserMinus size={12} />}
                        onClick={() => setConfirmRemove(u)} sx={{ fontSize: 11, height: 28, minWidth: 0, px: 1 }}>
                        Del
                      </Button>
                    </Box>
                  </TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </TableContainer>

      {/* Create User Dialog */}
      <Dialog open={createOpen} onClose={() => { if (!submitting) { setCreateOpen(false); setCreateError(''); setNewUser({ username: '', password: '' }) } }} maxWidth="xs" fullWidth>
        <DialogTitle>Create User</DialogTitle>
        <DialogContent>
          {createError && <Typography color="error" variant="body2" sx={{ mb: 1 }}>{createError}</Typography>}
          <TextField label="Username" size="small" fullWidth sx={{ mt: 1 }}
            value={newUser.username}
            onChange={(e) => setNewUser((p) => ({ ...p, username: e.target.value }))}
            disabled={submitting} autoFocus />
          <TextField label="Password" type="password" size="small" fullWidth sx={{ mt: 2 }}
            value={newUser.password}
            onChange={(e) => setNewUser((p) => ({ ...p, password: e.target.value }))}
            disabled={submitting} />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => { setCreateOpen(false); setCreateError('') }} disabled={submitting}>Cancel</Button>
          <Button variant="contained" onClick={handleCreate} disabled={submitting}>
            {submitting ? 'Creating...' : 'Create'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* Delete Confirmation Dialog */}
      <Dialog open={!!confirmRemove} onClose={() => setConfirmRemove(null)} maxWidth="xs">
        <DialogTitle>Remove {confirmRemove?.username}?</DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary">
            This will permanently remove the user. This action cannot be undone.
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirmRemove(null)}>Cancel</Button>
          <Button color="error" variant="contained" onClick={handleDelete}>Remove</Button>
        </DialogActions>
      </Dialog>

      {/* API Key Dialog */}
      <Dialog open={keyDialogOpen} onClose={() => setKeyDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>API Key — {generatedUser}</DialogTitle>
        <DialogContent>
          <Typography variant="caption2" color="warning.main" sx={{ mb: 1, display: 'block', fontWeight: 500 }}>
            This is the only time the plaintext API key will be shown. Full API access.
          </Typography>
          <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', mt: 1 }}>
            <TextField
              size="small"
              fullWidth
              value={keyVisible ? generatedKey : generatedKey.slice(0, 8) + '••••••••••••••••••••'}
              slotProps={{
                input: {
                  readOnly: true,
                  sx: { fontFamily: theme.typography.mono?.fontFamily ?? 'monospace', fontSize: 12 },
                },
              }}
            />
            <Button size="small" variant="outlined" onClick={() => setKeyVisible(!keyVisible)} sx={{ minWidth: 0, px: 1 }}>
              {keyVisible ? <EyeOff size={14} /> : <Eye size={14} />}
            </Button>
            <Button size="small" variant="outlined" onClick={handleCopy} sx={{ minWidth: 0, px: 1 }}>
              {copied ? <Check size={14} color="#10b981" /> : <Copy size={14} />}
            </Button>
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => { setKeyDialogOpen(false); setGeneratedKey('') }}>Close</Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
