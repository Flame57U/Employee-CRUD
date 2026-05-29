import { QueryClient } from '@tanstack/react-query'
import { ApiRequestError } from '@/lib/api'

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: (count, err) => {
        // Never retry auth failures — a 401 means the token is dead.
        if (err instanceof ApiRequestError && err.status === 401) return false
        return count < 1
      },
      refetchOnWindowFocus: false,
    },
  },
})
