import { DashboardLayout } from '@/components/DashboardLayout'
import { Card, CardContent } from '@/components/ui/card'
import { Activity, CheckCircle2, XCircle, Clock } from 'lucide-react'

export default function DashboardPage() {
  const getGreeting = () => {
    const hour = new Date().getHours()
    if (hour < 12) return 'Good morning'
    if (hour < 18) return 'Good afternoon'
    return 'Good evening'
  }

  return (
    <DashboardLayout>
      <div className="p-8">
        {/* Greeting */}
        <h1 className="text-2xl font-semibold mb-8">{getGreeting()}</h1>

        {/* Overview Section */}
        <div className="mb-6">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-lg font-medium">Overview</h2>
          </div>

          {/* Stats Grid */}
          <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-4">
            {/* Total Runs */}
            <Card>
              <CardContent className="p-6">
                <div className="flex items-center justify-between mb-4">
                  <p className="text-sm font-medium text-muted-foreground">Total Runs</p>
                  <Activity className="h-4 w-4 text-muted-foreground" />
                </div>
                <div className="space-y-1">
                  <p className="text-3xl font-semibold">-</p>
                  <p className="text-xs text-muted-foreground">No data yet</p>
                </div>
              </CardContent>
            </Card>

            {/* Passed */}
            <Card>
              <CardContent className="p-6">
                <div className="flex items-center justify-between mb-4">
                  <p className="text-sm font-medium text-muted-foreground">Passed</p>
                  <CheckCircle2 className="h-4 w-4 text-muted-foreground" />
                </div>
                <div className="space-y-1">
                  <p className="text-3xl font-semibold">-</p>
                  <p className="text-xs text-muted-foreground">No data yet</p>
                </div>
              </CardContent>
            </Card>

            {/* Failed */}
            <Card>
              <CardContent className="p-6">
                <div className="flex items-center justify-between mb-4">
                  <p className="text-sm font-medium text-muted-foreground">Failed</p>
                  <XCircle className="h-4 w-4 text-muted-foreground" />
                </div>
                <div className="space-y-1">
                  <p className="text-3xl font-semibold">-</p>
                  <p className="text-xs text-muted-foreground">No data yet</p>
                </div>
              </CardContent>
            </Card>

            {/* Avg Duration */}
            <Card>
              <CardContent className="p-6">
                <div className="flex items-center justify-between mb-4">
                  <p className="text-sm font-medium text-muted-foreground">Avg Duration</p>
                  <Clock className="h-4 w-4 text-muted-foreground" />
                </div>
                <div className="space-y-1">
                  <p className="text-3xl font-semibold">-</p>
                  <p className="text-xs text-muted-foreground">No data yet</p>
                </div>
              </CardContent>
            </Card>
          </div>
        </div>

        {/* Recent Activity */}
        <div>
          <h2 className="text-lg font-medium mb-4">Recent test runs</h2>
          <Card>
            <CardContent className="flex flex-col items-center justify-center py-16">
              <div className="flex h-16 w-16 items-center justify-center rounded-full bg-muted mb-4">
                <Activity className="h-8 w-8 text-muted-foreground" />
              </div>
              <h3 className="text-base font-medium mb-2">No test runs yet</h3>
              <p className="text-sm text-muted-foreground text-center max-w-sm">
                Run your first test to see results here
              </p>
            </CardContent>
          </Card>
        </div>
      </div>
    </DashboardLayout>
  )
}
