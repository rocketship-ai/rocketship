import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { RainbowButton } from '@/components/ui/rainbow-button'
import { useAuth } from './AuthContext'
import { tokenManager } from './tokenManager'

export default function OnboardingPage() {
  const navigate = useNavigate()
  const { checkAuth } = useAuth()
  const [step, setStep] = useState<'org' | 'verification'>('org')
  const [firstName, setFirstName] = useState('')
  const [lastName, setLastName] = useState('')
  const [orgName, setOrgName] = useState('')
  const [email, setEmail] = useState('')
  const [verificationCode, setVerificationCode] = useState('')
  const [registrationId, setRegistrationId] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleStartRegistration = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsLoading(true)
    setError(null)
    try {
      const response = await fetch('/api/orgs/registration/start', {
        method: 'POST',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          name: orgName,
          email: email,
        }),
      })

      if (!response.ok) {
        const errorData = await response.json()
        throw new Error(errorData.error || 'Failed to start registration')
      }

      const data = await response.json()
      setRegistrationId(data.registration_id)
      setStep('verification')
    } catch (error) {
      console.error('Failed to start registration:', error)
      setError(error instanceof Error ? error.message : 'Failed to start registration')
    } finally {
      setIsLoading(false)
    }
  }

  const handleCompleteRegistration = async (e: React.FormEvent) => {
    e.preventDefault()
    setIsLoading(true)
    setError(null)
    try {
      const response = await fetch('/api/orgs/registration/complete', {
        method: 'POST',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          registration_id: registrationId,
          code: verificationCode,
          first_name: firstName,
          last_name: lastName,
        }),
      })

      if (!response.ok) {
        const errorData = await response.json()
        throw new Error(errorData.error || 'Failed to complete registration')
      }

      const data = await response.json()

      // Server rotated tokens - force refresh the cached token before checking auth
      // This ensures we get the new JWT with updated org_id/name claims
      if (data.needs_claim_refresh) {
        await tokenManager.forceRefresh()
      }

      // Refresh auth state and navigate
      await checkAuth()
      navigate({ to: '/overview', replace: true })
    } catch (error) {
      console.error('Failed to complete registration:', error)
      setError(error instanceof Error ? error.message : 'Failed to complete registration')
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-background p-4">
      <div className="w-full max-w-md">
        {/* Logo */}
        <div className="text-center mb-4">
          <h1 className="text-2xl font-bold tracking-tight">Rocketship Cloud</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Agentic coding QA testing platform
          </p>
        </div>

        <div className="flex justify-center mb-6">
          <RainbowButton
            type="button"
            size="sm"
            className="px-3"
            onClick={() => navigate({ to: '/invites/accept' })}
          >
            Have an invite code?
          </RainbowButton>
        </div>

        {/* Progress indicator */}
        <div className="flex items-center justify-center mb-8">
          <div className="flex items-center gap-2">
            <div className={`flex h-8 w-8 items-center justify-center rounded-full text-xs font-medium ${
              step === 'verification' ? 'bg-primary text-primary-foreground' : 'bg-primary text-primary-foreground'
            }`}>
              {step === 'verification' ? '✓' : '1'}
            </div>
            <div className="w-16 h-px bg-border"></div>
            <div className={`flex h-8 w-8 items-center justify-center rounded-full text-xs font-medium ${
              step === 'verification' ? 'bg-primary text-primary-foreground' : 'bg-muted text-muted-foreground'
            }`}>
              2
            </div>
          </div>
        </div>

        {/* Error message */}
        {error && (
          <div className="mb-6 p-3 bg-red-950/50 border border-red-900 rounded-md">
            <p className="text-sm text-red-400">{error}</p>
          </div>
        )}

        {step === 'org' ? (
          <Card>
            <CardHeader className="space-y-1">
              <CardTitle className="text-2xl">Create organization</CardTitle>
              <CardDescription>
                Set up your workspace to get started
              </CardDescription>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleStartRegistration} className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <label htmlFor="firstName" className="text-sm font-medium">
                      First name
                    </label>
                    <Input
                      id="firstName"
                      type="text"
                      value={firstName}
                      onChange={(e) => setFirstName(e.target.value)}
                      placeholder="John"
                      required
                    />
                  </div>
                  <div className="space-y-2">
                    <label htmlFor="lastName" className="text-sm font-medium">
                      Last name
                    </label>
                    <Input
                      id="lastName"
                      type="text"
                      value={lastName}
                      onChange={(e) => setLastName(e.target.value)}
                      placeholder="Doe"
                      required
                    />
                  </div>
                </div>

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
                  We'll send a verification code to your email address
                </div>

                <Button
                  type="submit"
                  disabled={isLoading || !firstName.trim() || !lastName.trim() || !orgName.trim() || !email.trim()}
                  className="w-full"
                  size="lg"
                >
                  {isLoading ? 'Sending...' : 'Continue'}
                </Button>
              </form>
            </CardContent>
          </Card>
        ) : (
          <Card>
            <CardHeader className="space-y-1">
              <CardTitle className="text-2xl">Verify email</CardTitle>
              <CardDescription>
                Enter the verification code sent to {email}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleCompleteRegistration} className="space-y-4">
                <div className="space-y-2">
                  <label htmlFor="code" className="text-sm font-medium">
                    Verification code
                  </label>
                  <Input
                    id="code"
                    type="text"
                    value={verificationCode}
                    onChange={(e) => setVerificationCode(e.target.value)}
                    placeholder="Enter 6-digit code"
                    required
                    maxLength={6}
                  />
                </div>

                <div className="rounded-md bg-muted p-3 text-sm text-muted-foreground">
                  Check your email for the verification code
                </div>

                <Button
                  type="submit"
                  disabled={isLoading || !verificationCode.trim()}
                  className="w-full"
                  size="lg"
                >
                  {isLoading ? 'Verifying...' : 'Verify and continue'}
                </Button>
              </form>
            </CardContent>
          </Card>
        )}
      </div>
    </div>
  )
}
