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
import { ClipboardList } from 'lucide-react'

export default function RunsPage() {
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
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </div>
    </DashboardLayout>
  )
}
