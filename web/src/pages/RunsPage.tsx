import { DashboardLayout } from '@/components/DashboardLayout'
import { Card, CardContent } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ClipboardList, Loader2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { createClient } from '@connectrpc/connect'
import { createGrpcWebTransport } from '@connectrpc/connect-web'
import { Engine } from '@/generated/engine_pb'
import type { RunSummary, ListRunsRequest } from '@/generated/engine_pb'
import { useAuth } from '@/contexts/AuthContext'

export default function RunsPage() {
  const [runs, setRuns] = useState<RunSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const { accessToken } = useAuth()

  useEffect(() => {
    const fetchRuns = async () => {
      try {
        setLoading(true)
        setError(null)

        if (!accessToken) {
          setError('No access token available')
          return
        }

        // Create gRPC-Web transport (same-origin path through ingress)
        const transport = createGrpcWebTransport({
          baseUrl: '/engine',
          useBinaryFormat: true, // MUST use protobuf (server doesn't support JSON)
          // Add Authorization header with Bearer token
          interceptors: [
            (next) => async (req) => {
              req.header.set('Authorization', `Bearer ${accessToken}`)
              return await next(req)
            },
          ],
        })

        const client = createClient(Engine, transport)

        const req: ListRunsRequest = {
          limit: 50,
          descending: true,
        } as ListRunsRequest
        const res = await client.listRuns(req)

        setRuns(res.runs)
      } catch (err) {
        console.error('Failed to fetch runs:', err)
        setError(err instanceof Error ? err.message : 'Failed to fetch runs')
      } finally {
        setLoading(false)
      }
    }

    fetchRuns()
  }, [accessToken])

  const formatDuration = (durationMs: bigint) => {
    if (durationMs === 0n) return '-'
    const seconds = Number(durationMs) / 1000
    const minutes = Math.floor(seconds / 60)
    const remainingSeconds = Math.floor(seconds % 60)
    return minutes > 0
      ? `${minutes}m ${remainingSeconds}s`
      : `${remainingSeconds}s`
  }

  const formatTimestamp = (timestamp: string) => {
    if (!timestamp) return '-'
    const date = new Date(timestamp)
    return date.toLocaleString()
  }

  const getStatusColor = (status: string) => {
    switch (status.toLowerCase()) {
      case 'completed':
      case 'success':
        return 'text-green-600'
      case 'failed':
      case 'error':
        return 'text-red-600'
      case 'running':
        return 'text-blue-600'
      case 'cancelled':
        return 'text-gray-600'
      default:
        return 'text-gray-600'
    }
  }

  return (
    <DashboardLayout>
      <div className="p-8">
        <div className="mb-6">
          <h1 className="text-2xl font-semibold mb-2">Test runs</h1>
          <p className="text-sm text-muted-foreground">
            View and manage your test execution history
          </p>
        </div>

        <Card>
          <CardContent className="p-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Run ID</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Started</TableHead>
                  <TableHead>Duration</TableHead>
                  <TableHead>Tests</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading && (
                  <TableRow>
                    <TableCell colSpan={5} className="h-64">
                      <div className="flex flex-col items-center justify-center py-12">
                        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground mb-4" />
                        <p className="text-sm text-muted-foreground">
                          Loading test runs...
                        </p>
                      </div>
                    </TableCell>
                  </TableRow>
                )}

                {!loading && error && (
                  <TableRow>
                    <TableCell colSpan={5} className="h-64">
                      <div className="flex flex-col items-center justify-center py-12">
                        <div className="flex h-16 w-16 items-center justify-center rounded-full bg-red-100 mb-4">
                          <ClipboardList className="h-8 w-8 text-red-600" />
                        </div>
                        <p className="text-sm font-medium text-red-600">
                          Failed to load runs
                        </p>
                        <p className="text-sm text-muted-foreground mt-1">
                          {error}
                        </p>
                      </div>
                    </TableCell>
                  </TableRow>
                )}

                {!loading && !error && runs.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={5} className="h-64">
                      <div className="flex flex-col items-center justify-center py-12">
                        <div className="flex h-16 w-16 items-center justify-center rounded-full bg-muted mb-4">
                          <ClipboardList className="h-8 w-8 text-muted-foreground" />
                        </div>
                        <p className="text-sm font-medium">No test runs yet</p>
                        <p className="text-sm text-muted-foreground mt-1">
                          Run your first test to see results here
                        </p>
                      </div>
                    </TableCell>
                  </TableRow>
                )}

                {!loading &&
                  !error &&
                  runs.map((run) => (
                    <TableRow key={run.runId}>
                      <TableCell className="font-mono text-sm">
                        {run.runId.substring(0, 8)}
                      </TableCell>
                      <TableCell>
                        <span className={getStatusColor(run.status)}>
                          {run.status}
                        </span>
                      </TableCell>
                      <TableCell>{formatTimestamp(run.startedAt)}</TableCell>
                      <TableCell>{formatDuration(run.durationMs)}</TableCell>
                      <TableCell>
                        <span className="text-green-600">{run.passedTests}</span>
                        {' / '}
                        <span className="text-red-600">{run.failedTests}</span>
                        {' / '}
                        <span className="text-gray-600">{run.totalTests}</span>
                      </TableCell>
                    </TableRow>
                  ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </div>
    </DashboardLayout>
  )
}
