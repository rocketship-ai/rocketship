import { useEffect, useState } from "react";
import { Loader2, Check, Mail, Users, AlertCircle } from "lucide-react";
import {
  usePendingProjectInvites,
  useAcceptProjectInvite,
  useProjectInvitePreview,
} from "../hooks/use-console-queries";
import { useAuth } from "@/features/auth/AuthContext";
import { tokenManager } from "@/features/auth/tokenManager";

interface AcceptInviteProps {
  onComplete: () => void;
}

export function AcceptInvite({ onComplete }: AcceptInviteProps) {
  const searchParams = new URLSearchParams(window.location.search);
  const inviteFromQuery = searchParams.get("invite");
  const codeFromQuery = searchParams.get("code");

  const [code, setCode] = useState(codeFromQuery || "");
  const [inviteId, setInviteId] = useState(inviteFromQuery || "");
  const [error, setError] = useState<string | null>(null);
  const [acceptedInvite, setAcceptedInvite] = useState<{
    organization: { id: string; name: string };
    projects: { project_id: string; project_name: string; role: string }[];
  } | null>(null);

  const { checkAuth } = useAuth();
  const {
    data: pendingInvites = [],
    isLoading: invitesLoading,
    error: invitesError,
  } = usePendingProjectInvites();
  const {
    data: previewInvite,
    isLoading: previewLoading,
    error: previewError,
  } = useProjectInvitePreview(inviteId || null, code || null, {
    enabled: !!inviteId && !!code,
  });
  const acceptMutation = useAcceptProjectInvite();

  useEffect(() => {
    if (inviteFromQuery && inviteFromQuery !== inviteId) {
      setInviteId(inviteFromQuery);
    }
  }, [inviteFromQuery, inviteId]);

  useEffect(() => {
    if (codeFromQuery && codeFromQuery !== code) {
      setCode(codeFromQuery);
    }
  }, [codeFromQuery, code]);

  const handleAccept = async () => {
    if (!code.trim()) {
      setError("Please enter an invite code");
      return;
    }
    if (!inviteId.trim()) {
      setError(
        "Invite link is missing. Please use the invite link from your email."
      );
      return;
    }

    setError(null);
    try {
      const result = await acceptMutation.mutateAsync({
        invite_id: inviteId.trim(),
        code: code.trim(),
      });
      // Force refresh tokens and auth state after accepting invite
      // This ensures the user's status changes from 'pending' to 'ready'
      await tokenManager.forceRefresh();
      await checkAuth();
      setAcceptedInvite(result);
    } catch (err) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError("Failed to accept invite");
      }
    }
  };

  if (acceptedInvite) {
    return (
      <div className="min-h-screen bg-[#fafafa] flex items-center justify-center p-4">
        <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-8 max-w-md w-full text-center">
          <div className="w-16 h-16 rounded-full bg-green-100 flex items-center justify-center mx-auto mb-4">
            <Check className="w-8 h-8 text-green-600" />
          </div>
          <h1 className="text-xl font-semibold mb-2">Invite Accepted!</h1>
          <p className="text-[#666666] mb-4">
            You've joined <strong>{acceptedInvite.organization.name}</strong>
          </p>
          <div className="bg-[#fafafa] rounded-md p-4 mb-6 text-left">
            <p className="text-sm text-[#666666] mb-2">
              Projects you now have access to:
            </p>
            <ul className="space-y-1">
              {acceptedInvite.projects.map((p) => (
                <li
                  key={p.project_id}
                  className="flex items-center justify-between text-sm"
                >
                  <span>{p.project_name}</span>
                  <span
                    className={`px-2 py-0.5 rounded-full text-xs ${
                      p.role === "write"
                        ? "bg-green-50 text-green-700"
                        : "bg-blue-50 text-blue-700"
                    }`}
                  >
                    {p.role}
                  </span>
                </li>
              ))}
            </ul>
          </div>
          <button
            onClick={onComplete}
            className="w-full px-4 py-2 bg-black text-white rounded-md hover:bg-black/90 transition-colors"
          >
            Continue to Dashboard
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-[#fafafa] flex items-center justify-center p-4">
      <div className="bg-white rounded-lg border border-[#e5e5e5] shadow-sm p-8 max-w-lg w-full">
        <div className="text-center mb-6">
          <div className="w-16 h-16 rounded-full bg-[#f0f0f0] flex items-center justify-center mx-auto mb-4">
            <Users className="w-8 h-8 text-[#666666]" />
          </div>
          <h1 className="text-xl font-semibold mb-2">Accept Project Invite</h1>
          <p className="text-[#666666] text-sm">
            Enter the invite code from your email to join a project.
          </p>
        </div>

        {/* Pending invites section */}
        {invitesError ? (
          <div className="flex items-center gap-2 py-4 mb-6 px-3 bg-red-50 rounded-md border border-red-200">
            <AlertCircle className="w-5 h-5 text-red-600 flex-shrink-0" />
            <span className="text-sm text-red-700">
              Failed to load pending invites:{" "}
              {invitesError instanceof Error
                ? invitesError.message
                : "Unknown error"}
            </span>
          </div>
        ) : previewError ? (
          <div className="flex items-center gap-2 py-4 mb-6 px-3 bg-red-50 rounded-md border border-red-200">
            <AlertCircle className="w-5 h-5 text-red-600 flex-shrink-0" />
            <span className="text-sm text-red-700">
              Failed to load invite:{" "}
              {previewError instanceof Error
                ? previewError.message
                : "Unknown error"}
            </span>
          </div>
        ) : invitesLoading || previewLoading ? (
          <div className="flex items-center justify-center py-4 mb-6">
            <Loader2 className="w-5 h-5 animate-spin text-[#666666]" />
            <span className="ml-2 text-sm text-[#666666]">
              Checking for pending invites...
            </span>
          </div>
        ) : previewInvite ? (
          <div className="mb-6">
            <div className="flex items-center gap-2 mb-3">
              <Mail className="w-4 h-4 text-[#666666]" />
              <p className="text-sm text-[#666666]">
                Invite for <strong>{previewInvite.organization_name}</strong>:
              </p>
            </div>
            <div className="p-3 bg-[#fafafa] rounded-md border border-[#e5e5e5]">
              <p className="text-xs text-[#666666]">
                Invited by {previewInvite.inviter_name}
              </p>
              <div className="mt-2 flex flex-wrap gap-1">
                {previewInvite.projects.map((p) => (
                  <span
                    key={p.project_id}
                    className="inline-flex items-center gap-1 text-xs bg-white border border-[#e5e5e5] px-2 py-0.5 rounded"
                  >
                    {p.project_name}
                    <span
                      className={`font-medium ${
                        p.role === "write" ? "text-green-600" : "text-blue-600"
                      }`}
                    >
                      ({p.role})
                    </span>
                  </span>
                ))}
              </div>
            </div>
          </div>
        ) : pendingInvites.length > 0 ? (
          <div className="mb-6">
            <div className="flex items-center gap-2 mb-3">
              <Mail className="w-4 h-4 text-[#666666]" />
              <p className="text-sm text-[#666666]">
                You have {pendingInvites.length} pending invite
                {pendingInvites.length > 1 ? "s" : ""}:
              </p>
            </div>
            <div className="space-y-2">
              {pendingInvites.map((invite) => (
                <div
                  key={invite.id}
                  className="p-3 bg-[#fafafa] rounded-md border border-[#e5e5e5]"
                >
                  <p className="text-sm font-medium">
                    {invite.organization_name}
                  </p>
                  <p className="text-xs text-[#666666]">
                    Invited by {invite.inviter_name}
                  </p>
                  <div className="mt-2 flex flex-wrap gap-1">
                    {invite.projects.map((p) => (
                      <span
                        key={p.project_id}
                        className="inline-flex items-center gap-1 text-xs bg-white border border-[#e5e5e5] px-2 py-0.5 rounded"
                      >
                        {p.project_name}
                        <span
                          className={`font-medium ${
                            p.role === "write"
                              ? "text-green-600"
                              : "text-blue-600"
                          }`}
                        >
                          ({p.role})
                        </span>
                      </span>
                    ))}
                  </div>
                  <div className="mt-3">
                    <button
                      type="button"
                      onClick={() => setInviteId(invite.id)}
                      className={`text-xs px-3 py-1.5 rounded border transition-colors ${
                        inviteId === invite.id
                          ? "bg-black text-white border-black"
                          : "bg-white border-[#e5e5e5] text-[#666666] hover:bg-[#f5f5f5]"
                      }`}
                    >
                      {inviteId === invite.id ? "Selected" : "Use this invite"}
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </div>
        ) : null}

        {/* Code input */}
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-1">
              Invite Code
            </label>
            <input
              type="text"
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder="Enter your 6-digit code"
              className="w-full px-4 py-2 border border-[#e5e5e5] rounded-md focus:outline-none focus:ring-2 focus:ring-black/20 text-center text-lg tracking-widest"
              maxLength={6}
            />
          </div>

          {error && (
            <div className="text-sm text-red-600 bg-red-50 px-3 py-2 rounded-md">
              {error}
            </div>
          )}

          <button
            onClick={handleAccept}
            disabled={
              acceptMutation.isPending || !code.trim() || !inviteId.trim()
            }
            className="w-full flex items-center justify-center gap-2 px-4 py-2 bg-black text-white rounded-md hover:bg-black/90 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {acceptMutation.isPending ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin" />
                Accepting...
              </>
            ) : (
              "Accept Invite"
            )}
          </button>
        </div>

        <div className="mt-6 text-center">
          <button
            onClick={onComplete}
            className="text-sm text-[#666666] hover:text-black hover:underline"
          >
            Skip for now
          </button>
        </div>

        <p className="mt-6 text-center text-xs text-[#999999]">
          By accepting, you agree to our{" "}
          <a
            href="https://github.com/rocketship-ai/rocketship/blob/main/legal/rocketship-terms-of-service.md"
            target="_blank"
            rel="noopener noreferrer"
            className="underline hover:text-[#666666]"
          >
            Terms of Service
          </a>{" "}
          and{" "}
          <a
            href="https://github.com/rocketship-ai/rocketship/blob/main/legal/rocketship-privacy-policy.md"
            target="_blank"
            rel="noopener noreferrer"
            className="underline hover:text-[#666666]"
          >
            Privacy Policy
          </a>
        </p>
      </div>
    </div>
  );
}
