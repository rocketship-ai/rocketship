// Connect interceptor that automatically adds Authorization headers and retries on UNAUTHENTICATED

import type { Interceptor } from '@connectrpc/connect'
import { ConnectError, Code } from '@connectrpc/connect'
import { TokenManager } from './tokenManager'

export function authInterceptor(tm: TokenManager): Interceptor {
  return (next) => async (req) => {
    const token = await tm.get()
    if (token) req.header.set('Authorization', `Bearer ${token}`)

    try {
      return await next(req)
    } catch (e) {
      // If token expired mid-flight, refresh once and retry the call
      if (e instanceof ConnectError && e.code === Code.Unauthenticated) {
        const t2 = await tm.forceRefresh()
        if (t2) {
          req.header.set('Authorization', `Bearer ${t2}`)
          return await next(req)
        }
      }
      throw e
    }
  }
}
