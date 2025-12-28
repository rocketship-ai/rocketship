import { RouterProvider } from '@tanstack/react-router'
import { Toaster } from 'sonner'
import { router } from '@/app/router'

function App() {
  return (
    <>
      <RouterProvider router={router} />
      <Toaster
        position="top-right"
        toastOptions={{
          unstyled: true,
          classNames: {
            toast: 'bg-white border border-[#e5e5e5] rounded-lg shadow-md p-4 text-sm font-sans flex items-center gap-3',
          },
        }}
      />
    </>
  )
}

export default App
