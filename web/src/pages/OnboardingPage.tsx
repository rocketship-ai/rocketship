import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'

export default function OnboardingPage() {
  const navigate = useNavigate()
  const [step, setStep] = useState<'org' | 'email'>('org')
  const [orgName, setOrgName] = useState('')
  const [email, setEmail] = useState('')
  const [isLoading, setIsLoading] = useState(false)

  const handleCreateOrg = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsLoading(true)
    try {
      // TODO: Call API to create org
      await new Promise(resolve => setTimeout(resolve, 1000))
      setStep('email')
    } catch (error) {
      console.error('Failed to create org:', error)
    } finally {
      setIsLoading(false)
    }
  }

  const handleVerifyEmail = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsLoading(true)
    try {
      // TODO: Call API to verify email
      await new Promise(resolve => setTimeout(resolve, 1000))
      navigate('/dashboard')
    } catch (error) {
      console.error('Failed to verify email:', error)
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <div className="w-full max-w-md">
        {/* Logo */}
        <div className="text-center mb-8">
          <h1 className="text-2xl font-bold tracking-tight">Rocketship</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Professional testing platform
          </p>
        </div>

        {/* Progress indicator */}
        <div className="flex items-center justify-center mb-8">
          <div className="flex items-center gap-2">
            <div className={`flex h-8 w-8 items-center justify-center rounded-full text-xs font-medium ${
              step === 'email' ? 'bg-primary text-primary-foreground' : 'bg-primary text-primary-foreground'
            }`}>
              {step === 'email' ? '✓' : '1'}
            </div>
            <div className="w-16 h-px bg-border"></div>
            <div className={`flex h-8 w-8 items-center justify-center rounded-full text-xs font-medium ${
              step === 'email' ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground'
            }`}>
              2
            </div>
          </div>
        </div>

        {step === 'org' ? (
          <Card>
            <CardHeader className="space-y-1">
              <CardTitle className="text-2xl">Create organization</CardTitle>
              <CardDescription>
                Set up your workspace to get started
              </CardDescription>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleCreateOrg} className="space-y-4">
                <div className="space-y-2">
                  <label htmlFor="orgName" className="text-sm font-medium">
                    Organization name
                  </label>
                  <Input
                    id="orgName"
                    type="text"
                    value={orgName}
                    onChange={(e) => setOrgName(e.target.value)}
                    placeholder="Acme Inc"
                    required
                  />
                </div>

                <Button
                  type="submit"
                  disabled={isLoading || !orgName.trim()}
                  className="w-full"
                  size="lg"
                >
                  {isLoading ? 'Creating...' : 'Continue'}
                </Button>
              </form>
            </CardContent>
          </Card>
        ) : (
          <Card>
            <CardHeader className="space-y-1">
              <CardTitle className="text-2xl">Verify email</CardTitle>
              <CardDescription>
                Confirm your email address to continue
              </CardDescription>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleVerifyEmail} className="space-y-4">
                <div className="space-y-2">
                  <label htmlFor="email" className="text-sm font-medium">
                    Email address
                  </label>
                  <Input
                    id="email"
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="you@example.com"
                    required
                  />
                </div>

                <div className="rounded-md bg-muted p-3 text-sm text-muted-foreground">
                  We'll send a verification link to your email address
                </div>

                <Button
                  type="submit"
                  disabled={isLoading || !email.trim()}
                  className="w-full"
                  size="lg"
                >
                  {isLoading ? 'Sending...' : 'Send verification email'}
                </Button>
              </form>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  )
}
